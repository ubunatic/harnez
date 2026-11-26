package hookdispatch

import (
	"encoding/json"
	"strings"
	"testing"
)

// A deny must not carry a rewrite: Claude would run the rewritten input after
// the deny, which is the opposite of what the deny asked for.
func TestDenyClearsUpdatedInputAndNotesIt(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "rewrite", Kind: JSON, Run: reply(specific(t, map[string]any{"updatedInput": map[string]any{"command": "rewritten"}}))},
		{Name: "block", Kind: JSON, Run: reply(specific(t, map[string]any{"permissionDecision": "deny", "permissionDecisionReason": "no"}))},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Bash"}`)
	d := decode(t, got)
	if d.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("decision = %q, want deny", d.HookSpecificOutput.PermissionDecision)
	}
	if strings.Contains(got, "updatedInput") {
		t.Fatalf("deny kept a rewrite: %s", got)
	}
	if !strings.Contains(stderr, "updatedInput") {
		t.Fatalf("dropped rewrite under deny not noted: %q", stderr)
	}
}

// JSON null is not a rewrite. A null from one handler must not block the
// real rewrite of another, and a lone null must leave no updatedInput key.
func TestNullUpdatedInputIsNotARewrite(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "null", Kind: JSON, Run: reply(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":null,"additionalContext":"ctx"}}` + "\n")},
		{Name: "real", Kind: JSON, Run: reply(specific(t, map[string]any{"updatedInput": map[string]any{"command": "real"}}))},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Bash"}`)
	d := decode(t, got)
	if !strings.Contains(string(d.HookSpecificOutput.UpdatedInput), `"real"`) {
		t.Fatalf("real rewrite lost behind a null one: %s", got)
	}
	if strings.Contains(stderr, "second") {
		t.Fatalf("null was counted as a rewrite: %q", stderr)
	}

	lone := Registry{"PreToolUse": {
		{Name: "null", Kind: JSON, Run: reply(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":null}}` + "\n")},
		{Name: "ctx", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "ctx"}))},
	}}
	got, _ = run(t, lone, "PreToolUse", `{"tool_name":"Bash"}`)
	if strings.Contains(got, "updatedInput") {
		t.Fatalf("lone null produced an updatedInput key: %s", got)
	}
}

// `{}` is no reply. When one real reply is left, it passes through byte for byte.
func TestEmptyObjectReplyIsNoReply(t *testing.T) {
	want := specific(t, map[string]any{"additionalContext": "only"})
	reg := Registry{"PreToolUse": {
		{Name: "empty", Kind: JSON, Run: reply("{}\n")},
		{Name: "real", Kind: JSON, Run: reply(want)},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Read"}`)
	if got != want {
		t.Fatalf("real reply changed next to {}:\n got %q\nwant %q", got, want)
	}
	if stderr != "" {
		t.Fatalf("{} was logged as a dropped reply: %q", stderr)
	}
}

// Top-level keys the merge does not model (continue, stopReason, …) survive a
// merge. The first reply wins a conflict, and the conflict is noted.
func TestUnmodeledTopLevelKeysSurviveMerge(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "halt", Kind: JSON, Run: reply(`{"continue":false,"stopReason":"halt here"}` + "\n")},
		{Name: "ctx", Kind: JSON, Run: reply(`{"continue":true,"suppressOutput":true,"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"ctx"}}` + "\n")},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Read"}`)
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("output %q is not JSON: %v", got, err)
	}
	if string(doc["continue"]) != "false" {
		t.Fatalf("continue = %s, want false (first reply wins): %s", doc["continue"], got)
	}
	if string(doc["stopReason"]) != `"halt here"` {
		t.Fatalf("stopReason lost: %s", got)
	}
	if string(doc["suppressOutput"]) != "true" {
		t.Fatalf("suppressOutput from the second reply lost: %s", got)
	}
	if d := decode(t, got); d.HookSpecificOutput.AdditionalContext != "ctx" {
		t.Fatalf("modelled hookSpecificOutput lost: %s", got)
	}
	if !strings.Contains(stderr, "continue") {
		t.Fatalf("conflicting continue not noted: %q", stderr)
	}
}
