package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/sessionstate"
)

// TestMain isolates the tests from the agent environment harnez exports to the
// workers it starts: a developer, reviewer or advisor runs this suite with
// HARNEZ_AGENT_ROLE and HARNEZ_SESSION_ID set, and those must not change results.
// Tests that need them use t.Setenv. It also isolates tests from the real store
// (~/.harnez) by setting HOME to a temp dir, preventing session leaks.
func TestMain(m *testing.M) {
	os.Exit(runIsolatedTests(m))
}

func runIsolatedTests(m *testing.M) int {
	// Helpers inherit the parent's isolated HOME/runtime and own no temporary
	// directories. In particular, killed wrappers and os.Exit crash fixtures
	// cannot clean up a second TestMain's directories or a nested Go build.
	if os.Getenv("HARNEZ_STOP_FIXTURE") == "1" || os.Getenv("HARNEZ_WAIT_CRASH_HELPER") == "1" ||
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
	os.Unsetenv(agentSessionEnv)
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
	code := m.Run()

	if hadHome {
		os.Setenv("HOME", oldHome)
	} else {
		os.Unsetenv("HOME")
	}

	return code
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
