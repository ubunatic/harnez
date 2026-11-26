package claude

import (
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// legacyHarnezHooks are the 14 harnez entries apply wrote before issue 1304 M2:
// the config.yaml hooks plus spec/claude-compaction.yaml.
var legacyHarnezHooks = []Hook{
	{Event: "SessionStart", Command: "harnez hook claude-instructions"},
	{Event: "Stop", Command: "harnez hook sound"},
	{Event: "PreToolUse", Matcher: "Bash", Command: "harnez exec hook"},
	{Event: "PreToolUse", Matcher: "View", Command: "harnez hook read"},
	{Event: "PreToolUse", Matcher: "Read", Command: "harnez hook read"},
	{Event: "PreToolUse", Matcher: "ReadMultipleFiles", Command: "harnez hook read"},
	{Event: "PreToolUse", Matcher: "Edit", Command: "harnez hook pre-edit"},
	{Event: "PreToolUse", Matcher: "Write", Command: "harnez hook pre-edit"},
	{Event: "PostToolUse", Matcher: "Skill", Command: "harnez hook skill"},
	{Event: "PostToolUseFailure", Matcher: "Skill", Command: "harnez hook skill"},
	{Event: "UserPromptSubmit", Command: "harnez hook skill"},
	{Event: "PreCompact", Command: "harnez codex-telemetry"},
	{Event: "PreCompact", Command: "harnez hook compact-reset"},
	{Event: "PostCompact", Command: "harnez codex-telemetry"},
}

// settingsShape builds the settings hooks map the way buildSettingsDoc does.
func settingsShape(hooks []Hook) map[string]any {
	events := map[string]any{}
	for _, h := range hooks {
		entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": h.Command}}}
		if h.Matcher != "" {
			entry["matcher"] = h.Matcher
		}
		list, _ := events[h.Event].([]any)
		events[h.Event] = append(list, entry)
	}
	return events
}

// harnezCommandsIn lists every "event matcher command" triple whose command runs harnez.
func harnezCommandsIn(doc map[string]any) []Hook {
	var out []Hook
	hooks, _ := doc["hooks"].(map[string]any)
	for event, raw := range hooks {
		entries, _ := raw.([]any)
		for _, rawEntry := range entries {
			entry, _ := rawEntry.(map[string]any)
			matcher, _ := entry["matcher"].(string)
			inner, _ := entry["hooks"].([]any)
			for _, rawHook := range inner {
				h, _ := rawHook.(map[string]any)
				if command, _ := h["command"].(string); runsHarnez(command) {
					out = append(out, Hook{Event: event, Matcher: matcher, Command: command})
				}
			}
		}
	}
	sortHooks(out)
	return out
}

// runsHarnez is the test's own reading of "runs harnez": the first word's base
// name is harnez. It is deliberately separate from isHarnezCommand.
func runsHarnez(command string) bool {
	fields := strings.Fields(command)
	return len(fields) > 0 && path.Base(fields[0]) == "harnez"
}

func sortHooks(hooks []Hook) {
	sort.Slice(hooks, func(i, j int) bool {
		a, b := hooks[i], hooks[j]
		if a.Event != b.Event {
			return a.Event < b.Event
		}
		if a.Matcher != b.Matcher {
			return a.Matcher < b.Matcher
		}
		return a.Command < b.Command
	})
}

// A settings file from before M2 has 14 harnez entries, a foreign hook on the
// same event and an absolute-path harnez entry. One apply must leave exactly
// the 8 manifest entries and keep the foreign hook (issue 1304 M2, item 6).
func TestMigrationReplacesLegacyHarnezEntries(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	existing := settingsShape(legacyHarnezHooks)
	pre, _ := existing["PreToolUse"].([]any)
	pre = append(pre, map[string]any{"matcher": "Bash", "hooks": []any{map[string]any{"type": "command", "command": "/usr/local/bin/user-guard"}}})
	existing["PreToolUse"] = pre
	post, _ := existing["PostToolUse"].([]any)
	post = append(post, map[string]any{"matcher": "Skill", "hooks": []any{map[string]any{"type": "command", "command": "/home/u/go/bin/harnez hook skill"}}})
	existing["PostToolUse"] = post
	before := map[string]any{"hooks": existing}
	if n := len(harnezCommandsIn(before)); n != 15 {
		t.Fatalf("fixture has %d harnez entries, want 15 (14 legacy + absolute path)", n)
	}

	merged := applyMerge(before, buildSettingsDoc(cfg, nil))

	want := make([]Hook, 0, len(cfg.Hooks))
	want = append(want, cfg.Hooks...)
	sortHooks(want)
	if got := harnezCommandsIn(merged); !reflect.DeepEqual(got, want) {
		t.Fatalf("harnez entries after migration\n got %+v\nwant %+v", got, want)
	}
	if n := len(harnezCommandsIn(merged)); n != 8 {
		t.Fatalf("harnez entries = %d, want 8", n)
	}
	for _, legacy := range legacyHarnezHooks {
		if slicesContainsHook(harnezCommandsIn(merged), legacy) {
			t.Errorf("legacy entry survived: %+v", legacy)
		}
	}

	// The foreign hook on PreToolUse and its matcher survive.
	foreign := false
	hooks := merged["hooks"].(map[string]any)
	for _, raw := range hooks["PreToolUse"].([]any) {
		entry := raw.(map[string]any)
		for _, rawHook := range entry["hooks"].([]any) {
			if rawHook.(map[string]any)["command"] == "/usr/local/bin/user-guard" {
				foreign = true
			}
		}
	}
	if !foreign {
		t.Fatal("foreign PreToolUse hook was removed")
	}

	// A second apply changes nothing.
	if again := applyMerge(merged, buildSettingsDoc(cfg, nil)); !reflect.DeepEqual(again, merged) {
		t.Fatal("second apply after migration changed settings")
	}
}

func slicesContainsHook(hooks []Hook, h Hook) bool {
	for _, x := range hooks {
		if x == h {
			return true
		}
	}
	return false
}

func TestIsHarnezCommandMatchesBareAndAbsolutePaths(t *testing.T) {
	cases := map[string]bool{
		"harnez":                          true,
		"harnez hook Stop":                true,
		"/home/u/go/bin/harnez hook Stop": true,
		"  harnez exec hook  ":            true,
		"harnezfoo hook":                  false,
		"/usr/bin/harnez-channel serve":   false,
		"user-hook":                       false,
		"":                                false,
	}
	for command, want := range cases {
		if got := isHarnezCommand(command); got != want {
			t.Errorf("isHarnezCommand(%q) = %v, want %v", command, got, want)
		}
	}
}
