package hookdispatch

import "testing"

// RunsHarnez decides which host hook commands harnez owns, so apply can replace
// them and keep every foreign command (issue 1304 M2 follow-up 3).
func TestRunsHarnezTable(t *testing.T) {
	yes := []string{
		"harnez",
		"harnez hook Stop",
		"harnez codex-hook",
		"  harnez exec hook  ",
		"/home/u/go/bin/harnez hook Stop",
		"./harnez hook Stop",
		`"/home/u/go/bin/harnez" hook Stop`,
		`'harnez' hook Stop`,
		"env HARNEZ_TELEMETRY=1 harnez codex-telemetry",
		"env A=1 B=2 /usr/bin/harnez hook Stop",
		"HOME=/tmp harnez hook Stop",
		"harnez hook codex PreToolUse",
		// Quoted words with spaces are one word (issue 1304 M4 decision: POSIX quoting).
		`"/my dir/harnez" hook Stop`,
		`'/opt/bin/harnez' hook codex Stop`,
		`env A=1 B="x y" harnez hook Stop`,
		"harnez codex-telemetry",
	}
	no := []string{
		"",
		"   ",
		"harnezfoo hook",
		"harnez-foo hook",
		`"harnez-foo" hook`,
		"/usr/bin/harnez-channel serve",
		"/usr/local/bin/harnez-guard.sh",
		"/opt/harnez/run.sh",
		"sh -c harnez",
		"bash harnez hook Stop",
		"user-hook",
		"echo harnez",
		"env",
		"env HARNEZ=1",
		"HARNEZ=1",
		// env options are not parsed, so `env -i harnez ...` stays foreign.
		"env -i harnez hook Stop",
		// A quoted value that contains the word harnez is not the command word.
		`env MSG="hi harnez" /opt/user`,
		`FOO="a harnez" /opt/user`,
		`env harnez-foo hook`,
		`sh -c "harnez hook Stop"`,
		"/opt/harnez-wrap",
		`"/opt/harnez-wrap" hook Stop`,
		`'harnez-foo' hook`,
		// An unterminated quote is not a command we can read, so it stays foreign.
		`"harnez hook Stop`,
	}
	for _, c := range yes {
		if !RunsHarnez(c) {
			t.Errorf("RunsHarnez(%q) = false, want true", c)
		}
	}
	for _, c := range no {
		if RunsHarnez(c) {
			t.Errorf("RunsHarnez(%q) = true, want false", c)
		}
	}
}
