package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/sessionstate"
	"ubunatic.com/harnez/internal/usage"
)

var (
	testIsolatedHome    string
	testCommandGuardDir string
	testCommandGuardLog string
)

// TestMain isolates the tests from the agent environment harnez exports to the
// workers it starts: a developer, reviewer or advisor runs this suite with
// HARNEZ_AGENT_ROLE, HARNEZ_AGENT_MODEL and HARNEZ_SESSION_ID set, and those
// must not change results.
// Tests that need them use t.Setenv. It also isolates tests from the real store
// (~/.harnez) by setting HOME to a temp dir, preventing session leaks.
func TestMain(m *testing.M) {
	os.Exit(runIsolatedTests(m))
}

func runIsolatedTests(m *testing.M) int {
	// Helpers inherit the parent's isolated HOME/runtime and own no temporary
	// directories. In particular, killed wrappers and os.Exit crash fixtures
	// cannot clean up a second TestMain's directories or a nested Go build.
	if os.Getenv("TEST_983_CHILD") == "1" || os.Getenv("HARNEZ_STOP_FIXTURE") == "1" || os.Getenv("HARNEZ_WAIT_CRASH_HELPER") == "1" ||
		os.Getenv("HARNEZ_TEST_FOREGROUND_ID") != "" || os.Getenv("HARNEZ_TEST_WORKER_ID") != "" {
		return m.Run()
	}
	// A shared binary is created lazily by the tests that exercise it. Keep
	// its directory alive until all tests, including subprocesses, finish.
	defer func() { _ = os.RemoveAll(usageTestBinaryDir) }()
	if err := preserveGoCaches(); err != nil {
		panic(fmt.Sprintf("preserve Go caches: %v", err))
	}
	os.Unsetenv(agentRoleEnv)
	os.Unsetenv(agentModelEnv)
	os.Unsetenv(agentSessionEnv)
	os.Unsetenv(agentLauncherEnv)
	// The Pi agent dir would otherwise bypass the temp HOME and reach the real ~/.pi.
	os.Unsetenv("PI_CODING_AGENT_DIR")
	for _, name := range resolve.SessionEnvVars {
		os.Unsetenv(name)
	}
	// Host detection picks detach guidance wording (detachHost); keep it generic.
	for _, name := range []string{"HARNEZ_AGENT", "CODEX_THREAD_ID", "CODEX_CLI", "CLAUDE_CODE_SESSION_ID", "ANTIGRAVITY_AGENT", "ANTIGRAVITY_AGENTAPI_EXE"} {
		os.Unsetenv(name)
	}

	oldHome, hadHome := os.LookupEnv("HOME")
	tmpHome, err := os.MkdirTemp("", "harnez-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp HOME: %v", err))
	}
	defer os.RemoveAll(tmpHome)
	if err := os.Setenv("HOME", tmpHome); err != nil {
		panic(fmt.Sprintf("failed to set HOME: %v", err))
	}
	testIsolatedHome = tmpHome
	for name, dir := range map[string]string{
		"XDG_DATA_HOME":   "xdg-data",
		"XDG_CACHE_HOME":  "xdg-cache",
		"XDG_STATE_HOME":  "xdg-state",
		"XDG_CONFIG_HOME": "xdg-config",
	} {
		if err := os.Setenv(name, filepath.Join(tmpHome, dir)); err != nil {
			panic(fmt.Sprintf("failed to set isolated %s: %v", name, err))
		}
	}
	if err := installTestCommandGuards(tmpHome); err != nil {
		panic(fmt.Sprintf("install host command guards: %v", err))
	}
	tmpRuntime, err := os.MkdirTemp("", "harnez-runtime-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp XDG_RUNTIME_DIR: %v", err))
	}
	defer os.RemoveAll(tmpRuntime)
	if err := os.Setenv("XDG_RUNTIME_DIR", tmpRuntime); err != nil {
		panic(fmt.Sprintf("failed to set temp XDG_RUNTIME_DIR: %v", err))
	}

	// Without an agent session env var, resolve.Session falls back to a
	// PPID lock, so every in-process command would run the session-tip hook
	// against the telemetry DB. Tip-hook tests set their own options.
	sessionTipSessionOptions = func() resolve.SessionOptions {
		return resolve.SessionOptions{DisableFallback: true}
	}
	oldQuotaReader := agentTurnQuotaReader
	agentTurnQuotaReader = func(context.Context, string, bool, time.Time, time.Time) usage.TurnQuotaReading {
		return usage.TurnQuotaReading{}
	}
	code := m.Run()
	agentTurnQuotaReader = oldQuotaReader
	// Calls to these stubs are safe: they prove a test reached a deterministic
	// failure shim instead of a host provider binary. TestHostCommandGuardsAndXDGIsolation
	// verifies normal PATH lookup cannot resolve to the host tools.

	if hadHome {
		os.Setenv("HOME", oldHome)
	} else {
		os.Unsetenv("HOME")
	}

	return code
}

func installTestCommandGuards(home string) error {
	testCommandGuardDir = filepath.Join(home, "test-bin")
	if err := os.MkdirAll(testCommandGuardDir, 0700); err != nil {
		return err
	}
	testCommandGuardLog = filepath.Join(home, "guarded-host-commands.log")
	if err := os.Setenv("HARNEZ_TEST_COMMAND_GUARD_LOG", testCommandGuardLog); err != nil {
		return err
	}
	for _, name := range []string{"neus", "claude", "codex"} {
		script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s' >> \"$HARNEZ_TEST_COMMAND_GUARD_LOG\"\nprintf 'blocked host command: %%s\\n' '%s' >&2\nexit 97\n", name, name)
		if err := os.WriteFile(filepath.Join(testCommandGuardDir, name), []byte(script), 0700); err != nil {
			return err
		}
	}
	path := testCommandGuardDir
	if inherited := os.Getenv("PATH"); inherited != "" {
		path += string(os.PathListSeparator) + inherited
	}
	return os.Setenv("PATH", path)
}

func TestHostCommandGuardsAndXDGIsolation(t *testing.T) {
	if got := os.Getenv("HOME"); got != testIsolatedHome {
		t.Fatalf("HOME=%q, want isolated HOME %q", got, testIsolatedHome)
	}
	for _, name := range []string{"neus", "claude", "codex"} {
		got, err := exec.LookPath(name)
		want := filepath.Join(testCommandGuardDir, name)
		if err != nil || filepath.Clean(got) != filepath.Clean(want) {
			t.Fatalf("%s resolves to %q, %v; want guard %q", name, got, err, want)
		}
	}
	for _, name := range []string{"XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_CONFIG_HOME"} {
		got := os.Getenv(name)
		rel, err := filepath.Rel(testIsolatedHome, got)
		if err != nil || got == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			t.Fatalf("%s=%q escapes isolated HOME %q (relative %q, err %v)", name, got, testIsolatedHome, rel, err)
		}
	}
}

func TestEmbeddedTestRunnerSpecMatchesSchema(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/testing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/testing.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(document); err != nil {
		t.Fatalf("spec/testing.yaml does not match its schema: %v", err)
	}
}

func preserveGoCaches() error {
	cmd := exec.Command("go", "env", "GOCACHE", "GOMODCACHE")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	values := strings.Fields(string(output))
	if len(values) != 2 {
		return fmt.Errorf("go env returned %d cache paths, want 2", len(values))
	}
	if err := os.Setenv("GOCACHE", values[0]); err != nil {
		return err
	}
	return os.Setenv("GOMODCACHE", values[1])
}

func TestSessionTipHookSilentByDefault(t *testing.T) {
	oldOptions := sessionTipSessionOptions
	sessionTipSessionOptions = func() resolve.SessionOptions {
		return resolve.SessionOptions{DisableFallback: true}
	}
	t.Cleanup(func() { sessionTipSessionOptions = oldOptions })

	var stderr strings.Builder
	cmd := newRootCmd()
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("session tip stderr = %q, want empty under TestMain defaults", got)
	}
}

func TestSessionTipHookAllowsExplicitSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const sessionID = "explicit-tip-test"
	if err := sessionstate.Save(resolve.DefaultStateDir(), sessionstate.State{
		SessionID:   sessionID,
		Total:       40,
		Calls:       map[string]sessionstate.Invocation{},
		FirstCallAt: time.Now().Add(-24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(resolve.SessionEnvVars[0], sessionID)
	oldOptions := sessionTipSessionOptions
	sessionTipSessionOptions = func() resolve.SessionOptions {
		return resolve.SessionOptions{}
	}
	oldCounts := sessionTipCounts
	sessionTipCounts = func(string, string) map[string]int { return nil }
	t.Cleanup(func() {
		sessionTipSessionOptions = oldOptions
		sessionTipCounts = oldCounts
	})

	var stderr strings.Builder
	cmd := newRootCmd()
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := stderr.String(); !strings.Contains(got, "harnez tip:") {
		t.Fatalf("session tip stderr = %q, want a tip for explicit session", got)
	}
}
