package main

import (
	"os"
	"testing"
)

// TestMain isolates the tests from the agent environment harnez exports to the
// workers it starts: a developer, reviewer or advisor runs this suite with
// HARNEZ_AGENT_ROLE and HARNEZ_SESSION_ID set, and those must not change results.
// Tests that need them use t.Setenv.
func TestMain(m *testing.M) {
	os.Unsetenv(agentRoleEnv)
	os.Unsetenv(agentSessionEnv)
	os.Exit(m.Run())
}
