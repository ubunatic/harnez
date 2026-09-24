package main

import (
	"fmt"
	"os"
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
	os.Unsetenv(agentRoleEnv)
	os.Unsetenv(agentSessionEnv)
	for _, name := range resolve.SessionEnvVars {
		os.Unsetenv(name)
	}

	oldHome, hadHome := os.LookupEnv("HOME")
	tmpHome, err := os.MkdirTemp("", "harnez-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp HOME: %v", err))
	}
	if err := os.Setenv("HOME", tmpHome); err != nil {
		panic(fmt.Sprintf("failed to set HOME: %v", err))
	}
	tmpRuntime, err := os.MkdirTemp("", "harnez-runtime-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp XDG_RUNTIME_DIR: %v", err))
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", tmpRuntime); err != nil {
		panic(fmt.Sprintf("failed to set temp XDG_RUNTIME_DIR: %v", err))
	}

	code := m.Run()

	os.RemoveAll(tmpHome)
	os.RemoveAll(tmpRuntime)
	if hadHome {
		os.Setenv("HOME", oldHome)
	} else {
		os.Unsetenv("HOME")
	}

	os.Exit(code)
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
