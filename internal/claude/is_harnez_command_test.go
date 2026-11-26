package claude

import "testing"

// apply treats a hook command as harnez-owned by the same rule as
// hookdispatch.RunsHarnez: env assignments, quoting and absolute paths are
// harnez; harnez-* names and wrapper scripts are foreign (issue 1304 M3
// follow-up 3).
func TestIsHarnezCommandEnvQuotedAndNearMisses(t *testing.T) {
	for _, c := range []string{
		"harnez hook Stop",
		"/home/u/go/bin/harnez hook Stop",
		`"/home/u/go/bin/harnez" hook Stop`,
		"env HARNEZ_TELEMETRY=1 harnez codex-telemetry",
	} {
		if !isHarnezCommand(c) {
			t.Errorf("isHarnezCommand(%q) = false, want true", c)
		}
	}
	for _, c := range []string{
		"harnez-foo hook",
		"/usr/bin/harnez-channel serve",
		"/usr/local/bin/harnez-guard.sh",
		"user-hook",
		"sh -c harnez",
		"env HARNEZ=1",
		"",
	} {
		if isHarnezCommand(c) {
			t.Errorf("isHarnezCommand(%q) = true, want false", c)
		}
	}
}
