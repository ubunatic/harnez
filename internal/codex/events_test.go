package codex

import "testing"

func TestParseEventHookEnvelope(t *testing.T) {
	e := ParseEvent([]byte(`{"hook_event_name":"PostToolUse","session_id":"s","turn_id":"t","tool_use_id":"u","tool_name":"Bash","tool_input":{"command":"git status"}}`))
	if e.Kind != "posttooluse" || e.SessionID != "s" || e.ToolName != "Bash" || e.Command != "git status" { t.Fatalf("unexpected event: %+v", e) }
}

func TestParseEventTokenCountRollout(t *testing.T) {
	e := ParseEvent([]byte(`{"type":"event_msg","payload":{"session_id":"s","type":"token_count","info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3,"reasoning_tokens":2,"total_tokens":15}}}}`))
	if e.Kind != "token_count" || e.SessionID != "s" || e.TotalTokens == nil || *e.TotalTokens != 15 || e.ReasoningTokens == nil || *e.ReasoningTokens != 2 { t.Fatalf("unexpected token event: %+v", e) }
}

func TestParseEventMalformedAndUnknownAreSafe(t *testing.T) {
	if got := ParseEvent([]byte("not json")); got.Kind != "" { t.Fatalf("malformed event = %+v", got) }
	if got := ParseEvent([]byte(`{"type":"future_event","new_field":true}`)); got.Kind != "future_event" { t.Fatalf("unknown event = %+v", got) }
}
