package subagent

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestResolvePiModels(t *testing.T) {
	defaultModel, err := ResolveModel("pi")
	if err != nil {
		t.Fatal(err)
	}
	if defaultModel.Provider != "pi" || defaultModel.Name != "default" || defaultModel.Tier != "low" {
		t.Fatalf("default Pi model = %#v", defaultModel)
	}
	selected, err := ResolveModel("pi:OpenAI/gpt-5.6-Sol:high")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "pi" || selected.Name != "OpenAI/gpt-5.6-Sol" || selected.Tier != "high" {
		t.Fatalf("selected Pi model = %#v", selected)
	}
}

func TestPiRunAndResumeArgs(t *testing.T) {
	model := Model{Provider: "pi", Name: "openai/gpt-4o", Tier: "med"}
	if got, want := piRunArgs(model, "- inspect this"), []string{"--mode", "json", "--model", "openai/gpt-4o", "--thinking", "medium", "--", "- inspect this"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("piRunArgs() = %#v, want %#v", got, want)
	}
	if got, want := piResumeArgs("session-123", "continue", model), []string{"--mode", "json", "--session", "session-123", "--model", "openai/gpt-4o", "--thinking", "medium", "--", "continue"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("piResumeArgs() = %#v, want %#v", got, want)
	}
	if got, want := piRunArgs(Model{Provider: "pi", Name: "default", Tier: "low"}, "prompt"), []string{"--mode", "json", "--thinking", "low", "--", "prompt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default-model piRunArgs() = %#v, want %#v", got, want)
	}
}

func TestPiCompactUsesRPC(t *testing.T) {
	var gotCommand, gotInput string
	var gotArgs []string
	d := PiDriver{CommandInput: func(_ context.Context, command, input string, args ...string) ([]byte, error) {
		gotCommand, gotInput, gotArgs = command, input, args
		return []byte(`{"id":"harnez-compact","type":"response","command":"compact","success":true,"data":{"estimatedTokensAfter":1200,"usage":{"input":300,"output":40,"cacheRead":10,"cacheWrite":2}}}` + "\n"), nil
	}}
	r, err := d.Compact(context.Background(), "session-123")
	if err != nil {
		t.Fatal(err)
	}
	if gotCommand != "pi" || gotInput != "{\"id\":\"harnez-compact\",\"type\":\"compact\"}\n" || !reflect.DeepEqual(gotArgs, piCompactArgs("session-123")) {
		t.Fatalf("compact invocation = command %q input %q args %#v", gotCommand, gotInput, gotArgs)
	}
	if r.SessionID != "session-123" || r.ContextTokens != 1200 || !strings.Contains(r.Response, "compact") || r.TokensTurn != 352 {
		t.Fatalf("compact result = %#v", r)
	}
}

func TestParsePiCompactRejectsFailureOrMissingEstimate(t *testing.T) {
	failed := `{"id":"harnez-compact","type":"response","command":"compact","success":false,"error":"compaction failed"}`
	if _, err := parsePiCompact([]byte(failed)); err == nil || !strings.Contains(err.Error(), "compaction failed") {
		t.Fatalf("parsePiCompact(failed) error = %v", err)
	}
	missing := `{"id":"harnez-compact","type":"response","command":"compact","success":true,"data":{}}`
	if _, err := parsePiCompact([]byte(missing)); err == nil || !strings.Contains(err.Error(), "estimatedTokensAfter") {
		t.Fatalf("parsePiCompact(missing estimate) error = %v", err)
	}
}

func TestPiDriverRunAndResume(t *testing.T) {
	var calls [][]string
	d := PiDriver{Command: func(_ context.Context, command string, args ...string) ([]byte, error) {
		if command != "pi" {
			t.Fatalf("command = %q, want pi", command)
		}
		calls = append(calls, args)
		return []byte(piJSONOutput("session-123", "done")), nil
	}}
	model := Model{Provider: "pi", Name: "anthropic/claude-sonnet-4-5", Tier: "high"}
	r, err := d.Run(context.Background(), RunOptions{Prompt: "work", Model: model})
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "session-123" || r.Response != "done" {
		t.Fatalf("Run() result = %#v", r)
	}
	if _, err := d.Resume(context.Background(), "session-123", "follow-up", model); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !reflect.DeepEqual(calls[0], piRunArgs(model, "work")) || !reflect.DeepEqual(calls[1], piResumeArgs("session-123", "follow-up", model)) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestParsePiJSONEvents(t *testing.T) {
	r, err := parsePi([]byte(piJSONOutput("session-123", "first") + piAssistantEvent("second")))
	if err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "session-123" || r.Response != "second" || !reflect.DeepEqual(r.Messages, []string{"first", "second"}) {
		t.Fatalf("result = %#v", r)
	}
	if r.InputTokens != 20 || r.OutputTokens != 8 || r.CachedTokens != 6 || r.ContextTokens != 13 || r.TokensTurn != 34 {
		t.Fatalf("usage = %#v", r)
	}
}

func TestParsePiAllowsRecoveryAfterFailedAssistantMessage(t *testing.T) {
	failed := `{"type":"session","id":"s"}` + "\n" + `{"type":"message_end","message":{"role":"assistant","stopReason":"error","errorMessage":"transient failure"}}` + "\n"
	got, err := parsePi([]byte(failed + piAssistantEvent("recovered") + `{"type":"agent_settled"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Response != "recovered" {
		t.Fatalf("response = %q, want recovered result", got.Response)
	}
}

func TestParsePiRejectsFailedOrIncompleteOutput(t *testing.T) {
	failed := `{"type":"session","id":"s"}` + "\n" + `{"type":"message_end","message":{"role":"assistant","stopReason":"error","errorMessage":"provider failed"}}` + "\n" + `{"type":"agent_settled"}` + "\n"
	if _, err := parsePi([]byte(failed)); err == nil || !strings.Contains(err.Error(), "provider failed") {
		t.Fatalf("parsePi(failed) error = %v", err)
	}
	if _, err := parsePi([]byte(`{"type":"session","id":"s"}` + "\n" + piAssistantEvent("answer"))); err == nil || !strings.Contains(err.Error(), "did not settle") {
		t.Fatalf("parsePi(unsettled) error = %v", err)
	}
	if _, err := parsePi([]byte(`{"type":"agent_settled"}`)); err == nil || !strings.Contains(err.Error(), "no session header") {
		t.Fatalf("parsePi(incomplete) error = %v", err)
	}
}

func piJSONOutput(sessionID, response string) string {
	return `{"type":"session","id":"` + sessionID + `"}` + "\n" + piAssistantEvent(response) + `{"type":"agent_settled"}` + "\n"
}

func piAssistantEvent(response string) string {
	return `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"` + response + `"}],"stopReason":"stop","usage":{"input":10,"output":4,"cacheRead":2,"cacheWrite":1,"totalTokens":17}}}` + "\n"
}
