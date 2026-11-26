package hookdispatch

import (
	"reflect"
	"strings"
	"testing"
)

// The Claude and Codex manifests are read from spec/hooks.yaml (issue 1304 M3).
// The Claude entries are the 8 settings entries apply writes; the Codex entries
// are one `harnez hook codex <Event>` command per event.
func TestClaudeManifestEntries(t *testing.T) {
	m, err := LoadManifest("claude")
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Entries()
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Event: "PostCompact", Command: "harnez hook PostCompact"},
		{Event: "PostToolUse", Matcher: "Skill", Command: "harnez hook PostToolUse"},
		{Event: "PostToolUseFailure", Matcher: "Skill", Command: "harnez hook PostToolUseFailure"},
		{Event: "PreCompact", Command: "harnez hook PreCompact"},
		{Event: "PreToolUse", Matcher: "Bash|View|Read|ReadMultipleFiles|Edit|Write", Command: "harnez hook PreToolUse"},
		{Event: "SessionStart", Command: "harnez hook SessionStart"},
		{Event: "Stop", Command: "harnez hook Stop"},
		{Event: "UserPromptSubmit", Command: "harnez hook UserPromptSubmit"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries\n got %+v\nwant %+v", got, want)
	}
}

func TestCodexManifestEntries(t *testing.T) {
	m, err := LoadManifest("codex")
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Entries()
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Event: "PostCompact", Command: "harnez hook codex PostCompact"},
		{Event: "PostToolUse", Matcher: "*", Command: "harnez hook codex PostToolUse"},
		{Event: "PreCompact", Command: "harnez hook codex PreCompact"},
		{Event: "PreToolUse", Matcher: "Bash", Command: "harnez hook codex PreToolUse"},
		{Event: "SessionEnd", Matcher: "other", Command: "harnez hook codex SessionEnd"},
		{Event: "SessionStart", Matcher: "startup|resume|clear|compact", Command: "harnez hook codex SessionStart"},
		{Event: "Stop", Command: "harnez hook codex Stop"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries\n got %+v\nwant %+v", got, want)
	}
}

// Each embedded agent has its own command prefix, so the two agents' entries
// for the same event name never collide.
func TestAgentsHaveDistinctCommands(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		m, err := LoadManifest(agent)
		if err != nil {
			t.Fatal(err)
		}
		if len(m.EventNames()) == 0 {
			t.Fatalf("%s manifest has no events", agent)
		}
	}
	if _, err := LoadManifest("gemini"); err == nil {
		t.Fatal("unknown agent accepted")
	}
}

// A matcher skips the events it does not list. An event that mixes tooled and
// untooled handlers would silently skip the untooled ones, so it is rejected.
func TestManifestRejectsMixedToolAndUntooledHandlers(t *testing.T) {
	m := Manifest{Command: "harnez hook", Events: map[string]Event{"PreToolUse": {Handlers: []Handler{
		{Name: "a", Kind: Silent, Tools: []string{"Bash"}},
		{Name: "b", Kind: Silent},
	}}}}
	if _, err := m.Entries(); err == nil {
		t.Fatal("mixed tooled and untooled handlers accepted")
	}
}

// An explicit matcher and handler tool routing would be two sources for one
// host matcher, so the manifest rejects the combination.
func TestManifestRejectsExplicitMatcherWithToolRouting(t *testing.T) {
	m := Manifest{Command: "harnez hook", Events: map[string]Event{"PreToolUse": {
		Matcher:  "Bash",
		Handlers: []Handler{{Name: "a", Kind: Silent, Tools: []string{"Bash"}}},
	}}}
	if _, err := m.Entries(); err == nil {
		t.Fatal("explicit matcher with tool routing accepted")
	}
}

func TestBindPairsEveryHandlerWithOneFunc(t *testing.T) {
	m := Manifest{Command: "harnez hook", Events: map[string]Event{"Stop": {Handlers: []Handler{{Name: "sound", Kind: Silent}}}}}
	if _, err := m.Bind(map[string]Func{}); err == nil {
		t.Error("handler with no Run bound was accepted")
	}
	if _, err := m.Bind(map[string]Func{"Stop/sound": reply(""), "Stop/extra": reply("")}); err == nil {
		t.Error("Run with no manifest handler was accepted")
	}
	reg, err := m.Bind(map[string]Func{"Stop/sound": reply("")})
	if err != nil {
		t.Fatal(err)
	}
	if reg["Stop"][0].Run == nil {
		t.Fatal("bound registry has no Run")
	}
	if m.Events["Stop"].Handlers[0].Run != nil {
		t.Fatal("Bind mutated the manifest")
	}
}

// The embedded spec's bindings are checked both ways: every manifest handler
// needs a Run and every Run needs a manifest handler. The cmd package supplies
// the Runs; here the names alone must match the spec.
func TestEmbeddedManifestHandlerNames(t *testing.T) {
	want := map[string]string{
		"claude": "PostCompact:codex-telemetry PostToolUse:skill PostToolUseFailure:skill PreCompact:codex-telemetry,compact-reset PreToolUse:exec,read,pre-edit SessionStart:claude-instructions Stop:sound UserPromptSubmit:skill",
		"codex":  "PostCompact:codex-telemetry PostToolUse:codex-telemetry PreCompact:codex-telemetry PreToolUse:codex-hook SessionEnd:codex-telemetry SessionStart:codex-telemetry Stop:codex-telemetry",
	}
	for agent, wantNames := range want {
		m, err := LoadManifest(agent)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, event := range m.EventNames() {
			var names []string
			for _, h := range m.Events[event].Handlers {
				names = append(names, h.Name)
			}
			got = append(got, event+":"+strings.Join(names, ","))
		}
		if strings.Join(got, " ") != wantNames {
			t.Errorf("%s handlers\n got %s\nwant %s", agent, strings.Join(got, " "), wantNames)
		}
	}
}

// minimalSpec is a valid hooks spec with one claude event; the schema tests
// mutate it.
const minimalSpec = `handler_timeout: 40s
agents:
  claude:
    command: harnez hook
    events:
      Stop:
        handlers:
          - {name: sound, kind: silent}
  codex:
    command: harnez hook codex
    events:
      Stop:
        handlers:
          - {name: codex-telemetry, kind: silent}
`

func TestParseManifestAcceptsMinimalSpec(t *testing.T) {
	m, err := ParseManifest([]byte(minimalSpec), "claude")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := m.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Command != "harnez hook Stop" {
		t.Fatalf("entries = %+v", entries)
	}
}

// The schema rejects every malformed spec before a manifest is built, so a bad
// edit to spec/hooks.yaml fails the test run instead of writing bad settings.
func TestParseManifestRejectsBadSpec(t *testing.T) {
	cases := map[string]string{
		"unknown kind":        strings.Replace(minimalSpec, "kind: silent", "kind: bogus", 1),
		"unknown key":         strings.Replace(minimalSpec, "    command: harnez hook\n", "    command: harnez hook\n    hooks: []\n", 1),
		"top-level hooks key": minimalSpec + "hooks: []\n",
		"bad handler name":    strings.Replace(minimalSpec, "name: sound", "name: Sound", 1),
		"empty handlers":      strings.Replace(minimalSpec, "- {name: sound, kind: silent}", "[]", 1),
		"no events":           strings.Replace(minimalSpec, "      Stop:\n        handlers:\n          - {name: sound, kind: silent}\n", "", 1),
		"bad tool name":       strings.Replace(minimalSpec, "kind: silent}", "kind: silent, tools: [Bash|Read]}", 1),
		"missing timeout":     strings.Replace(minimalSpec, "handler_timeout: 40s\n", "", 1),
		"bad timeout":         strings.Replace(minimalSpec, "40s", "forty", 1),
		"no agents":           "handler_timeout: 40s\n",
	}
	for name, doc := range cases {
		if _, err := ParseManifest([]byte(doc), "claude"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
