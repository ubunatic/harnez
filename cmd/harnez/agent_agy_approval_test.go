package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

// Read-only agy roles run tools unattended (issue 995): start and resume pass
// the role's auto_approve_tools to the agy driver; developer does not.
func TestAgentAgyRoleAutoApprovesTools(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	for _, tc := range []struct {
		role string
		auto bool
	}{
		{"advisor", true},
		{"reviewer", true},
		{"developer", false},
	} {
		t.Run(tc.role, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			storeDir, workDir := t.TempDir(), t.TempDir()
			var calls [][]string
			driver := subagent.AgyDriver{Dir: workDir, Command: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				calls = append(calls, args)
				return []byte(`{"conversation_id":"agy-c1","status":"SUCCESS","response":"done","usage":{"output_tokens":3}}`), nil
			}}
			if _, err := runWithStore(t, driver, storeDir, "start", "--name", "worker", "--role", tc.role, "--model", "argon", "review the diff"); err != nil {
				t.Fatal(err)
			}
			if _, err := runWithStore(t, driver, storeDir, "resume", "--name", "worker", "go on"); err != nil {
				t.Fatal(err)
			}
			if len(calls) != 2 {
				t.Fatalf("agy calls = %d, want start and resume", len(calls))
			}
			for i, args := range calls {
				gotApprove := slices.Contains(args, "--dangerously-skip-permissions") && slices.Contains(args, "--sandbox")
				if gotApprove != tc.auto {
					t.Errorf("call %d args = %q; auto-approve = %t, want %t", i, args, gotApprove, tc.auto)
				}
				if args[len(args)-2] != "-p" {
					t.Errorf("call %d args = %q; -p must precede the prompt last", i, args)
				}
			}
			if !strings.HasPrefix(strings.Join(calls[1], " "), "--conversation agy-c1 ") {
				t.Fatalf("resume args = %q, want the provider conversation", calls[1])
			}
		})
	}
}
