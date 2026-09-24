package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

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

	oldHome := os.Getenv("HOME")
	tmpHome, err := os.MkdirTemp("", "harnez-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp HOME: %v", err))
	}
	if err := os.Setenv("HOME", tmpHome); err != nil {
		panic(fmt.Sprintf("failed to set HOME: %v", err))
	}

	code := m.Run()

	os.RemoveAll(tmpHome)
	if oldHome != "" {
		os.Setenv("HOME", oldHome)
	}

	os.Exit(code)
}

func TestSessionTipHookSilentByDefault(t *testing.T) {
	sessionID, err := resolve.Session(resolve.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := sessionstate.Save(resolve.DefaultStateDir(), sessionstate.State{
		SessionID: sessionID,
		Total:     40,
		Calls:     map[string]sessionstate.Invocation{},
	}); err != nil {
		t.Fatal(err)
	}

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
