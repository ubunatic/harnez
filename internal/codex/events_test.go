package codex

import "testing"

func TestParseEventHookEnvelope(t *testing.T) {
	e := ParseEvent([]byte(`{"hook_event_name":"PostToolUse","session_id":"s","turn_id":"t","tool_use_id":"u","tool_name":"Bash","tool_input":{"command":"git status"}}`))
	if e.Kind != "posttooluse" || e.SessionID != "s" || e.ToolName != "Bash" || e.Command != "git status" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestParseEventTokenCountRollout(t *testing.T) {
	e := ParseEvent([]byte(`{"type":"event_msg","payload":{"session_id":"s","type":"token_count","info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3,"reasoning_output_tokens":2,"total_tokens":15},"last_token_usage":{"input_tokens":6,"cached_input_tokens":2,"output_tokens":3,"reasoning_output_tokens":1,"total_tokens":10}}}}`))
	if e.Kind != "token_count" || e.SessionID != "s" || e.TotalTokens == nil || *e.TotalTokens != 15 || e.ReasoningTokens == nil || *e.ReasoningTokens != 2 {
		t.Fatalf("unexpected token event: %+v", e)
	}
	if e.LastTotalTokens == nil || *e.LastTotalTokens != 10 || e.LastReasoningTokens == nil || *e.LastReasoningTokens != 1 {
		t.Fatalf("unexpected per-turn token event: %+v", e)
	}
}

func TestParseEventPreToolUse(t *testing.T) {
	e := ParseEvent([]byte(`{"hook_event_name":"PreToolUse","session_id":"s","turn_id":"t","tool_use_id":"u","tool_name":"Bash","tool_input":{"command":"ls"}}`))
	if e.Kind != "pretooluse" || e.SessionID != "s" || e.ToolCallID != "u" || e.Command != "ls" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestParseEventToolCallRollout(t *testing.T) {
	e := ParseEvent([]byte(`{"type":"event_msg","payload":{"session_id":"s","type":"tool_call"}}`))
	if e.Kind != "tool_call" || e.SessionID != "s" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestParseEventSessionBoundaries(t *testing.T) {
	start := ParseEvent([]byte(`{"hook_event_name":"SessionStart","session_id":"s"}`))
	if start.Kind != "sessionstart" || start.SessionID != "s" {
		t.Fatalf("unexpected session start event: %+v", start)
	}
	end := ParseEvent([]byte(`{"type":"event_msg","payload":{"session_id":"s","type":"session_end"}}`))
	if end.Kind != "session_end" || end.SessionID != "s" {
		t.Fatalf("unexpected session end event: %+v", end)
	}
}

func TestParseEventFailureHookEnvelope(t *testing.T) {
	e := ParseEvent([]byte(`{"hook_event_name":"PostToolUse","session_id":"s","tool_use_id":"u","tool_name":"Bash","tool_input":{"command":"false"}}`))
	if e.Kind != "posttooluse" || e.Command != "false" {
		t.Fatalf("unexpected event: %+v", e)
	}
	// Success/ExitCode classification from the failure payload is M2 scope
	// (see issue 420); this fixture only pins the shape ParseEvent must accept.
}

func TestParseEventMalformedAndUnknownAreSafe(t *testing.T) {
	if got := ParseEvent([]byte("not json")); got.Kind != "" {
		t.Fatalf("malformed event = %+v", got)
	}
	if got := ParseEvent([]byte(`{"type":"future_event","new_field":true}`)); got.Kind != "future_event" {
		t.Fatalf("unknown event = %+v", got)
	}
}

func TestParseEventResultFields(t *testing.T) {
	e := ParseEvent([]byte(`{"hook_event_name":"PostToolUse","session_id":"s","tool_name":"Bash","success":false,"exit_code":2,"duration_ms":17,"tool_output":"failed"}`))
	if e.Success == nil || *e.Success || e.ExitCode == nil || *e.ExitCode != 2 || e.DurationMs != 17 || e.OutputBytes == nil || *e.OutputBytes != 6 {
		t.Fatalf("unexpected result fields: %+v", e)
	}
}
