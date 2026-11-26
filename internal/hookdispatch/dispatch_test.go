package hookdispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

const testBudget = 5 * time.Second

// reply returns a handler that drains its payload and writes text.
func reply(text string) Func {
	return func(_ context.Context, in io.Reader, out io.Writer) error {
		_, _ = io.Copy(io.Discard, in)
		_, err := io.WriteString(out, text)
		return err
	}
}

// specific builds one Claude hookSpecificOutput reply as JSON.
func specific(t *testing.T, fields map[string]any) string {
	t.Helper()
	fields["hookEventName"] = "PreToolUse"
	b, err := json.Marshal(map[string]any{"hookSpecificOutput": fields})
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func run(t *testing.T, reg Registry, event, payload string) (out, stderr string) {
	t.Helper()
	var o, e bytes.Buffer
	if err := Dispatch(context.Background(), reg, event, []byte(payload), &o, Options{Budget: testBudget, Stderr: &e}); err != nil {
		t.Fatalf("Dispatch(%s): %v", event, err)
	}
	return o.String(), e.String()
}

type decoded struct {
	HookSpecificOutput struct {
		HookEventName            string          `json:"hookEventName"`
		PermissionDecision       string          `json:"permissionDecision"`
		PermissionDecisionReason string          `json:"permissionDecisionReason"`
		UpdatedInput             json.RawMessage `json:"updatedInput"`
		AdditionalContext        string          `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
	SystemMessage string `json:"systemMessage"`
}

func decode(t *testing.T, s string) decoded {
	t.Helper()
	var d decoded
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		t.Fatalf("output %q is not JSON: %v", s, err)
	}
	return d
}

func TestSingleReplyPassesThroughVerbatim(t *testing.T) {
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no"}}` + "\n"
	reg := Registry{"PreToolUse": {{Name: "a", Kind: JSON, Run: reply(want)}}}
	got, _ := run(t, reg, "PreToolUse", `{"tool_name":"Edit"}`)
	if got != want {
		t.Fatalf("single reply changed:\n got %q\nwant %q", got, want)
	}
}

func TestDenyWinsOverAllowAndKeepsWinnerReason(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "allow", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "ctx-a"}))},
		{Name: "deny", Kind: JSON, Run: reply(specific(t, map[string]any{"permissionDecision": "deny", "permissionDecisionReason": "blocked by b"}))},
	}}
	got, _ := run(t, reg, "PreToolUse", `{"tool_name":"Edit"}`)
	d := decode(t, got)
	if d.HookSpecificOutput.PermissionDecision != "deny" || d.HookSpecificOutput.PermissionDecisionReason != "blocked by b" {
		t.Fatalf("deny did not win with its reason: %+v", d.HookSpecificOutput)
	}
	if d.HookSpecificOutput.AdditionalContext != "ctx-a" {
		t.Fatalf("allow handler's additionalContext was lost: %+v", d.HookSpecificOutput)
	}
}

func TestDecisionPrecedenceAndTieKeepsFirst(t *testing.T) {
	cases := []struct {
		name    string
		seq     []string // decisions in registry order
		want    string
		wantWhy string
	}{
		{"ask beats allow", []string{"allow", "ask"}, "ask", "reason-1"},
		{"defer beats ask", []string{"ask", "defer", "allow"}, "defer", "reason-1"},
		{"deny beats defer", []string{"defer", "deny"}, "deny", "reason-1"},
		{"tie keeps first deny", []string{"deny", "deny"}, "deny", "reason-0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var handlers []Handler
			for i, dec := range tc.seq {
				fields := map[string]any{"permissionDecision": dec, "permissionDecisionReason": "reason-" + string(rune('0'+i))}
				handlers = append(handlers, Handler{Name: dec, Kind: JSON, Run: reply(specific(t, fields))})
			}
			got, _ := run(t, Registry{"PreToolUse": handlers}, "PreToolUse", `{"tool_name":"Edit"}`)
			d := decode(t, got)
			if d.HookSpecificOutput.PermissionDecision != tc.want {
				t.Fatalf("decision = %q, want %q", d.HookSpecificOutput.PermissionDecision, tc.want)
			}
			if d.HookSpecificOutput.PermissionDecisionReason != tc.wantWhy {
				t.Fatalf("reason = %q, want %q", d.HookSpecificOutput.PermissionDecisionReason, tc.wantWhy)
			}
		})
	}
}

func TestSecondUpdatedInputIsDroppedAndLogged(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "first", Kind: JSON, Run: reply(specific(t, map[string]any{"updatedInput": map[string]any{"command": "first"}}))},
		{Name: "second", Kind: JSON, Run: reply(specific(t, map[string]any{"updatedInput": map[string]any{"command": "second"}}))},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Bash"}`)
	d := decode(t, got)
	if !strings.Contains(string(d.HookSpecificOutput.UpdatedInput), `"first"`) || strings.Contains(got, `"second"`) {
		t.Fatalf("expected only the first rewrite, got %s", got)
	}
	if !strings.Contains(stderr, "second") {
		t.Fatalf("dropped rewrite not logged to stderr: %q", stderr)
	}
}

func TestAdditionalContextJoinedInOrder(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "a", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "one"}))},
		{Name: "b", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "two"}))},
		{Name: "c", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "three"}))},
	}}
	got, _ := run(t, reg, "PreToolUse", `{"tool_name":"Read"}`)
	if d := decode(t, got); d.HookSpecificOutput.AdditionalContext != "one\ntwo\nthree" {
		t.Fatalf("additionalContext = %q", d.HookSpecificOutput.AdditionalContext)
	}
}

func TestSystemMessageJoinedAndEmptyRepliesGiveEmptyObject(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "a", Kind: JSON, Run: reply(`{"systemMessage":"warn a"}` + "\n")},
		{Name: "b", Kind: JSON, Run: reply(`{}` + "\n")},
		{Name: "c", Kind: JSON, Run: reply(`{"systemMessage":"warn c"}` + "\n")},
	}}
	got, _ := run(t, reg, "PreToolUse", `{"tool_name":"Read"}`)
	if d := decode(t, got); d.SystemMessage != "warn a\nwarn c" {
		t.Fatalf("systemMessage = %q", d.SystemMessage)
	}
	empty := Registry{"PreToolUse": {
		{Name: "a", Kind: JSON, Run: reply("{}\n")},
		{Name: "b", Kind: JSON, Run: reply("{}\n")},
	}}
	if got, _ := run(t, empty, "PreToolUse", `{"tool_name":"Read"}`); got != "{}\n" {
		t.Fatalf("two empty replies = %q, want {}", got)
	}
}

func TestTextEventJoinsWithNewline(t *testing.T) {
	reg := Registry{"SessionStart": {
		{Name: "a", Kind: Text, Run: reply("A\n")},
		{Name: "b", Kind: Text, Run: reply("B")},
	}}
	if got, _ := run(t, reg, "SessionStart", ``); got != "A\nB\n" {
		t.Fatalf("text join = %q", got)
	}
}

func TestPanicDropsOnlyThatHandler(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "panics", Kind: JSON, Run: func(context.Context, io.Reader, io.Writer) error { panic("boom") }},
		{Name: "ok", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "from-ok"}))},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Edit"}`)
	if d := decode(t, got); d.HookSpecificOutput.AdditionalContext != "from-ok" {
		t.Fatalf("surviving handler output lost: %q", got)
	}
	if !strings.Contains(stderr, "panics") {
		t.Fatalf("panic not noted on stderr: %q", stderr)
	}
}

func TestTimeoutDropsOnlyThatHandler(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	reg := Registry{"PreToolUse": {
		{Name: "stuck", Kind: JSON, Run: func(context.Context, io.Reader, io.Writer) error {
			<-block // ignores ctx on purpose: the dispatcher must not wait for it
			return nil
		}},
		{Name: "ok", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "from-ok"}))},
	}}
	var o, e bytes.Buffer
	err := Dispatch(context.Background(), reg, "PreToolUse", []byte(`{"tool_name":"Edit"}`), &o, Options{Budget: 100 * time.Millisecond, Stderr: &e})
	if err != nil {
		t.Fatalf("Dispatch returned %v, want nil", err)
	}
	if d := decode(t, o.String()); d.HookSpecificOutput.AdditionalContext != "from-ok" {
		t.Fatalf("surviving handler output lost: %q", o.String())
	}
	if !strings.Contains(e.String(), "stuck") {
		t.Fatalf("timeout not noted on stderr: %q", e.String())
	}
}

func TestErrorDropsOutputOfThatHandler(t *testing.T) {
	reg := Registry{"PreToolUse": {
		{Name: "fails", Kind: JSON, Run: func(_ context.Context, _ io.Reader, out io.Writer) error {
			_, _ = io.WriteString(out, `{"hookSpecificOutput":{"permissionDecision":"deny"}}`)
			return io.ErrUnexpectedEOF
		}},
		{Name: "ok", Kind: JSON, Run: reply(specific(t, map[string]any{"additionalContext": "from-ok"}))},
	}}
	got, stderr := run(t, reg, "PreToolUse", `{"tool_name":"Edit"}`)
	if strings.Contains(got, "deny") {
		t.Fatalf("failed handler's partial output leaked: %q", got)
	}
	if !strings.Contains(stderr, "fails") {
		t.Fatalf("error not noted on stderr: %q", stderr)
	}
}

func TestHandlerOrderIsRegistryOrder(t *testing.T) {
	names := []string{"e", "a", "d", "b", "c"}
	for round := 0; round < 3; round++ {
		var seen []string
		var handlers []Handler
		for _, n := range names {
			handlers = append(handlers, Handler{Name: n, Kind: Silent, Run: func(_ context.Context, _ io.Reader, _ io.Writer) error {
				seen = append(seen, n)
				return nil
			}})
		}
		run(t, Registry{"PreCompact": handlers}, "PreCompact", `{}`)
		if !reflect.DeepEqual(seen, names) {
			t.Fatalf("round %d: order = %v, want %v", round, seen, names)
		}
	}
}

func TestToolRoutingSkipsOtherTools(t *testing.T) {
	var ran []string
	mark := func(name string) Func {
		return func(context.Context, io.Reader, io.Writer) error { ran = append(ran, name); return nil }
	}
	reg := Registry{"PreToolUse": {
		{Name: "bash-only", Kind: Silent, Tools: []string{"Bash"}, Run: mark("bash-only")},
		{Name: "edit-write", Kind: Silent, Tools: []string{"Edit", "Write"}, Run: mark("edit-write")},
		{Name: "every-tool", Kind: Silent, Run: mark("every-tool")},
	}}
	run(t, reg, "PreToolUse", `{"tool_name":"Edit"}`)
	if want := []string{"edit-write", "every-tool"}; !reflect.DeepEqual(ran, want) {
		t.Fatalf("ran %v, want %v", ran, want)
	}
}

func TestSilentHandlerOutputIsDropped(t *testing.T) {
	reg := Registry{"Stop": {
		{Name: "quiet", Kind: Silent, Run: reply("junk\n")},
		{Name: "other", Kind: Silent, Run: reply("")},
	}}
	got, stderr := run(t, reg, "Stop", `{}`)
	if got != "" {
		t.Fatalf("silent handler wrote %q", got)
	}
	if !strings.Contains(stderr, "quiet") {
		t.Fatalf("dropped silent output not noted: %q", stderr)
	}
}

func TestUntooledHandlersRunWithoutToolName(t *testing.T) {
	reg := Registry{"UserPromptSubmit": {
		{Name: "skill", Kind: Silent, Run: reply("")},
	}}
	var ran bool
	reg["UserPromptSubmit"][0].Run = func(context.Context, io.Reader, io.Writer) error { ran = true; return nil }
	run(t, reg, "UserPromptSubmit", `{"prompt":"hi"}`)
	if !ran {
		t.Fatal("handler without Tools did not run for a payload with no tool_name")
	}
}

func TestValidateRejectsBadRegistries(t *testing.T) {
	ok := func(name string, kind Kind) Handler {
		return Handler{Name: name, Kind: kind, Run: reply("")}
	}
	cases := map[string]Registry{
		"mixed JSON and Text": {"SessionStart": {ok("a", JSON), ok("b", Text)}},
		"empty name":          {"Stop": {{Kind: Silent, Run: reply("")}}},
		"nil run":             {"Stop": {{Name: "x", Kind: Silent}}},
		"unknown kind":        {"Stop": {{Name: "x", Kind: Kind(99), Run: reply("")}}},
	}
	for name, reg := range cases {
		if err := reg.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
	silentWithJSON := Registry{"PreToolUse": {ok("a", JSON), ok("b", Silent)}}
	if err := silentWithJSON.Validate(); err != nil {
		t.Errorf("Silent mixed with JSON should be allowed: %v", err)
	}
}

func TestDispatchRejectsUnknownEventAndBadBudget(t *testing.T) {
	reg := Registry{"Stop": {{Name: "x", Kind: Silent, Run: reply("")}}}
	var o bytes.Buffer
	if err := Dispatch(context.Background(), reg, "NoSuchEvent", nil, &o, Options{Budget: testBudget}); err == nil {
		t.Error("unknown event was accepted")
	}
	if err := Dispatch(context.Background(), reg, "Stop", nil, &o, Options{}); err == nil {
		t.Error("zero budget was accepted")
	}
}
