package hookdispatch

import (
	"reflect"
	"testing"
)

// The Claude manifest is the one place that says which events harnez installs,
// which tools each one matches and what each entry runs (issue 1304 M2). These
// are the 8 entries `apply` writes, one per event.
func TestClaudeManifestEntries(t *testing.T) {
	got, err := ClaudeManifest.Entries()
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

// A matcher skips the events it does not list. An event that mixes tooled and
// untooled handlers would silently skip the untooled ones, so it is rejected.
func TestManifestRejectsMixedToolAndUntooledHandlers(t *testing.T) {
	m := Manifest{"PreToolUse": {
		{Name: "a", Kind: Silent, Tools: []string{"Bash"}},
		{Name: "b", Kind: Silent},
	}}
	if _, err := m.Entries(); err == nil {
		t.Fatal("mixed tooled and untooled handlers accepted")
	}
}

func TestBindPairsEveryHandlerWithOneFunc(t *testing.T) {
	m := Manifest{"Stop": {{Name: "sound", Kind: Silent}}}
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
	if m["Stop"][0].Run != nil {
		t.Fatal("Bind mutated the manifest")
	}
}
