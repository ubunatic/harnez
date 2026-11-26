package codex

import "testing"

// Codex gets one `harnez hook codex <Event>` entry per event. The matcher and
// command come from spec/hooks.yaml (issue 1304 M3).
func TestBuildHooksDocEntriesMatchCodexManifest(t *testing.T) {
	want := []struct{ event, matcher, command string }{
		{"PostCompact", "", "harnez hook codex PostCompact"},
		{"PostToolUse", "*", "harnez hook codex PostToolUse"},
		{"PreCompact", "", "harnez hook codex PreCompact"},
		{"PreToolUse", "Bash", "harnez hook codex PreToolUse"},
		{"SessionEnd", "other", "harnez hook codex SessionEnd"},
		{"SessionStart", "startup|resume|clear|compact", "harnez hook codex SessionStart"},
		{"Stop", "", "harnez hook codex Stop"},
	}
	hooks := BuildHooksDoc()["hooks"].(map[string]any)
	if len(hooks) != len(want) {
		t.Fatalf("events = %d, want %d: %v", len(hooks), len(want), hooks)
	}
	for _, w := range want {
		groups := tableArray(hooks[w.event])
		if len(groups) != 1 {
			t.Errorf("%s: groups = %d, want 1", w.event, len(groups))
			continue
		}
		matcher, _ := groups[0]["matcher"].(string)
		if matcher != w.matcher {
			t.Errorf("%s: matcher = %q, want %q", w.event, matcher, w.matcher)
		}
		handlers := tableArray(groups[0]["hooks"])
		if len(handlers) != 1 || handlers[0]["command"] != w.command || handlers[0]["type"] != "command" {
			t.Errorf("%s: handlers = %v, want one command %q", w.event, handlers, w.command)
		}
	}
}
