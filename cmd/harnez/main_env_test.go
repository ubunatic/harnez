package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates the tests from the agent environment harnez exports to the
// workers it starts: a developer, reviewer or advisor runs this suite with
// HARNEZ_AGENT_ROLE and HARNEZ_SESSION_ID set, and those must not change results.
// Tests that need them use t.Setenv. It also isolates tests from the real store
// (~/.harnez) by setting HOME to a temp dir, preventing session leaks.
func TestMain(m *testing.M) {
	os.Unsetenv(agentRoleEnv)
	os.Unsetenv(agentSessionEnv)

	oldHome := os.Getenv("HOME")
	tmpHome, err := os.MkdirTemp("", "harnez-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp HOME: %v", err))
	}
	defer os.RemoveAll(tmpHome)
	if err := os.Setenv("HOME", tmpHome); err != nil {
		panic(fmt.Sprintf("failed to set HOME: %v", err))
	}
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
		}
	}()

	code := m.Run()
	os.Exit(code)
}
