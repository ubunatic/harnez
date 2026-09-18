package codex

import (
	"encoding/json"
	"strings"
)

// Event is the normalized subset of a Codex lifecycle record needed by the
// telemetry adapter. Codex has added fields to rollout records over time, so
// parsing deliberately keeps unknown fields harmless and optional values nil.
type Event struct {
	Kind                                                                                           string
	SessionID                                                                                      string
	TurnID                                                                                         string
	ToolCallID                                                                                     string
	ToolName                                                                                       string
	Command                                                                                        string
	Success                                                                                        *bool
	ExitCode                                                                                       *int
	DurationMs                                                                                     int64
	OutputBytes                                                                                    *int64
	InputTokens, CachedInputTokens, OutputTokens, ReasoningTokens, TotalTokens                     *int64
	LastInputTokens, LastCachedInputTokens, LastOutputTokens, LastReasoningTokens, LastTotalTokens *int64
	CompactionTrigger, CompactionReason                                                            string
}

// ParseEvent normalizes one Codex rollout JSON object. It accepts both the
// native hook envelope names and the rollout event_msg/token_count shapes;
// unsupported or malformed records return an empty event without error so a
// tailing adapter can safely process mixed-version transcripts.
func ParseEvent(raw []byte) Event {
	var envelope struct {
		Type             string          `json:"type"`
		Event            string          `json:"event"`
		Hook             string          `json:"hook_event_name"`
		HookCamel        string          `json:"hookEventName"`
		SessionID        string          `json:"session_id"`
		TurnID           string          `json:"turn_id"`
		ToolCallID       string          `json:"tool_use_id"`
		ToolName         string          `json:"tool_name"`
		ToolInput        json.RawMessage `json:"tool_input"`
		Payload          json.RawMessage `json:"payload"`
		ToolOutput       json.RawMessage `json:"tool_output"`
		Trigger          string          `json:"trigger"`
		Reason           string          `json:"reason"`
		CompactionReason string          `json:"compaction_reason"`
		TokenUsage       json.RawMessage `json:"token_usage"`
		LastTokenUsage   json.RawMessage `json:"last_token_usage"`
		TotalTokenUsage  json.RawMessage `json:"total_token_usage"`
		Usage            json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return Event{}
	}
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	e := Event{Kind: strings.ToLower(strings.TrimSpace(first(envelope.Hook, envelope.HookCamel, envelope.Event, envelope.Type))), SessionID: envelope.SessionID, TurnID: envelope.TurnID, ToolCallID: envelope.ToolCallID, ToolName: envelope.ToolName}
	e.CompactionTrigger = first(envelope.Trigger, stringValue(fields, "compaction_trigger"))
	e.CompactionReason = first(envelope.Reason, envelope.CompactionReason, stringValue(fields, "reason"))
	if len(envelope.TokenUsage) == 0 {
		envelope.TokenUsage = envelope.Usage
	}
	if len(envelope.TokenUsage) > 0 {
		parseTokenSnapshot(envelope.TokenUsage, &e.InputTokens, &e.CachedInputTokens, &e.OutputTokens, &e.ReasoningTokens, &e.TotalTokens)
	}
	if len(envelope.TotalTokenUsage) > 0 {
		parseTokenSnapshot(envelope.TotalTokenUsage, &e.InputTokens, &e.CachedInputTokens, &e.OutputTokens, &e.ReasoningTokens, &e.TotalTokens)
	}
	if len(envelope.LastTokenUsage) > 0 {
		parseTokenSnapshot(envelope.LastTokenUsage, &e.LastInputTokens, &e.LastCachedInputTokens, &e.LastOutputTokens, &e.LastReasoningTokens, &e.LastTotalTokens)
	}
	if e.Kind == "event_msg" && len(envelope.Payload) > 0 {
		var p struct {
			SessionID string `json:"session_id"`
			Type      string `json:"type"`
			Info      struct {
				Total struct {
					Input           int64 `json:"input_tokens"`
					Cached          int64 `json:"cached_input_tokens"`
					Output          int64 `json:"output_tokens"`
					Reasoning       int64 `json:"reasoning_tokens"`
					ReasoningOutput int64 `json:"reasoning_output_tokens"`
					Total           int64 `json:"total_tokens"`
				} `json:"total_token_usage"`
				Last struct {
					Input           int64 `json:"input_tokens"`
					Cached          int64 `json:"cached_input_tokens"`
					Output          int64 `json:"output_tokens"`
					Reasoning       int64 `json:"reasoning_tokens"`
					ReasoningOutput int64 `json:"reasoning_output_tokens"`
					Total           int64 `json:"total_tokens"`
				} `json:"last_token_usage"`
			} `json:"info"`
		}
		if json.Unmarshal(envelope.Payload, &p) == nil {
			e.SessionID = first(p.SessionID, e.SessionID)
			if p.Type != "" {
				e.Kind = strings.ToLower(p.Type)
			}
			t := p.Info.Total
			reasoning := t.Reasoning
			if reasoning == 0 {
				reasoning = t.ReasoningOutput
			}
			e.InputTokens, e.CachedInputTokens, e.OutputTokens, e.ReasoningTokens, e.TotalTokens = ptr(t.Input), ptr(t.Cached), ptr(t.Output), ptr(reasoning), ptr(t.Total)
			lastReasoning := p.Info.Last.Reasoning
			if lastReasoning == 0 {
				lastReasoning = p.Info.Last.ReasoningOutput
			}
			e.LastInputTokens, e.LastCachedInputTokens, e.LastOutputTokens, e.LastReasoningTokens, e.LastTotalTokens = ptr(p.Info.Last.Input), ptr(p.Info.Last.Cached), ptr(p.Info.Last.Output), ptr(lastReasoning), ptr(p.Info.Last.Total)
		}
	}
	if len(envelope.ToolInput) > 0 {
		var input struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(envelope.ToolInput, &input) == nil {
			e.Command = input.Command
		}
	}
	if value, ok := fields["success"].(bool); ok {
		e.Success = &value
	}
	if value, ok := fields["is_error"].(bool); ok {
		value = !value
		e.Success = &value
	}
	if value, ok := fields["exit_code"].(float64); ok {
		code := int(value)
		e.ExitCode = &code
		success := code == 0
		e.Success = &success
	}
	if value, ok := fields["duration_ms"].(float64); ok && value >= 0 {
		e.DurationMs = int64(value)
	}
	if len(envelope.ToolOutput) > 0 {
		var output string
		if json.Unmarshal(envelope.ToolOutput, &output) == nil {
			bytes := int64(len(output))
			e.OutputBytes = &bytes
		} else if size := len(envelope.ToolOutput); size > 0 {
			bytes := int64(size)
			e.OutputBytes = &bytes
		}
	}
	return e
}

func stringValue(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func parseTokenSnapshot(raw []byte, input, cached, output, reasoning, total **int64) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	*input = optionalInt(fields, "input_tokens")
	*cached = optionalInt(fields, "cached_input_tokens")
	*output = optionalInt(fields, "output_tokens")
	*reasoning = optionalInt(fields, "reasoning_tokens")
	if *reasoning == nil {
		*reasoning = optionalInt(fields, "reasoning_output_tokens")
	}
	*total = optionalInt(fields, "total_tokens")
}

func optionalInt(fields map[string]json.RawMessage, key string) *int64 {
	raw, ok := fields[key]
	if !ok {
		return nil
	}
	var value int64
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return &value
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
