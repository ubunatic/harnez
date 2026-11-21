package main

import "testing"

func TestPluginStatusReadsTheBlockOfEachPlugin(t *testing.T) {
	// Shape of real `claude plugin list` output (2026-11-21).
	list := `Installed plugins:

  ❯ harnez-channel@harnez-local
    Version: 0.5.9
    Scope: user
    Status: ✔ enabled

  ❯ jev-compaction@harnez-local
    Version: 0.5.9
    Status: ✘ disabled

  ❯ no-status@harnez-local
    Version: 0.5.9

  ❯ other@x
    Status: ✔ enabled
`
	for _, tc := range []struct {
		plugin string
		want   status
	}{
		{"harnez-channel@harnez-local", statusEnabled},
		{"jev-compaction@harnez-local", statusDisabled},
		{"no-status@harnez-local", statusDisabled},
		{"absent@harnez-local", statusMissing},
	} {
		if got := pluginStatus(list, tc.plugin); got != tc.want {
			t.Errorf("pluginStatus(%s) = %d, want %d", tc.plugin, got, tc.want)
		}
	}
}
