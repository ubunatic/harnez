package codex

import (
	"encoding/json"
	"strings"
)

// Event is the normalized subset of a Codex lifecycle record needed by the
// telemetry adapter. Codex has added fields to rollout records over time, so
// parsing deliberately keeps unknown fields harmless and optional values nil.
type Event struct {
	Kind       string
	SessionID  string
	TurnID     string
	ToolCallID string
	ToolName   string
	Command    string
	Success    *bool
	ExitCode   *int
	InputTokens, CachedInputTokens, OutputTokens, ReasoningTokens, TotalTokens *int64
}

// ParseEvent normalizes one Codex rollout JSON object. It accepts both the
// native hook envelope names and the rollout event_msg/token_count shapes;
// unsupported or malformed records return an empty event without error so a
// tailing adapter can safely process mixed-version transcripts.
func ParseEvent(raw []byte) Event {
	var envelope struct {
		Type       string          `json:"type"`
		Event      string          `json:"event"`
		Hook       string          `json:"hook_event_name"`
		SessionID  string          `json:"session_id"`
		TurnID     string          `json:"turn_id"`
		ToolCallID string          `json:"tool_use_id"`
		ToolName   string          `json:"tool_name"`
		ToolInput  json.RawMessage `json:"tool_input"`
		Payload    json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return Event{}
	}
	e := Event{Kind: strings.ToLower(strings.TrimSpace(first(envelope.Hook, envelope.Event, envelope.Type))), SessionID: envelope.SessionID, TurnID: envelope.TurnID, ToolCallID: envelope.ToolCallID, ToolName: envelope.ToolName}
	if e.Kind == "event_msg" && len(envelope.Payload) > 0 {
		var p struct {
			SessionID string `json:"session_id"`
			Type string `json:"type"`
			Info struct { Total struct { Input int64 `json:"input_tokens"`; Cached int64 `json:"cached_input_tokens"`; Output int64 `json:"output_tokens"`; Reasoning int64 `json:"reasoning_tokens"`; Total int64 `json:"total_tokens"` } `json:"total_token_usage"` } `json:"info"`
		}
		if json.Unmarshal(envelope.Payload, &p) == nil {
			e.SessionID = first(p.SessionID, e.SessionID)
			if p.Type != "" { e.Kind = strings.ToLower(p.Type) }
			t := p.Info.Total
			e.InputTokens, e.CachedInputTokens, e.OutputTokens, e.ReasoningTokens, e.TotalTokens = ptr(t.Input), ptr(t.Cached), ptr(t.Output), ptr(t.Reasoning), ptr(t.Total)
		}
	}
	if len(envelope.ToolInput) > 0 {
		var input struct { Command string `json:"command"` }
		if json.Unmarshal(envelope.ToolInput, &input) == nil { e.Command = input.Command }
	}
	return e
}

func first(values ...string) string { for _, value := range values { if strings.TrimSpace(value) != "" { return value } }; return "" }
func ptr(value int64) *int64 { if value == 0 { return nil }; return &value }
