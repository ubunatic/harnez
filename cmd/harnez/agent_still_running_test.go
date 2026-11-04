package main

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

// Issue 1111: a Jules wait that runs out while the session still runs exits 3,
// like harnez pr wait's "still open at the timeout"; other results keep their code.
func TestStillRunningExit(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name string
		r    *subagent.TurnResult
		err  error
		want int
	}{
		{"still running exits 3", &subagent.TurnResult{StillRunning: true}, nil, 3},
		{"finished turn exits 0", &subagent.TurnResult{}, nil, 0},
		{"nil result exits 0", nil, nil, 0},
		{"print error wins over still running", &subagent.TurnResult{StillRunning: true}, boom, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			err := stillRunningExit(cmd, tc.r, tc.err)
			if got := exitCodeFromRunError(err); got != tc.want {
				t.Fatalf("exit code = %d, want %d (err %v)", got, tc.want, err)
			}
			if tc.want == 3 && (!cmd.SilenceErrors || !cmd.SilenceUsage) {
				t.Error("exit 3 must be silent: the result was already printed")
			}
		})
	}
}
