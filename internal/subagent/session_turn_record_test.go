package subagent

import (
	"encoding/json"
	"testing"
)

func TestTurnRecordReasoningTokensRetention(t *testing.T) {
	// Test that reasoning tokens are kept in saved session
	r1 := TurnRecord{Turn: 1, ReasoningTokens: 42}
	b, err := json.Marshal(r1)
	if err != nil {
		t.Fatalf("Failed to marshal TurnRecord: %v", err)
	}
	var r2 TurnRecord
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("Failed to unmarshal TurnRecord: %v", err)
	}
	if r2.ReasoningTokens != 42 {
		t.Errorf("ReasoningTokens: got %d, want 42", r2.ReasoningTokens)
	}
}

func TestTurnRecordLegacyCompatibility(t *testing.T) {
	// Test that old session JSON without reasoning tokens still loads
	oldJSON := `{"turn": 1, "new_input_tokens": 10, "cached_input_tokens": 20, "output_tokens": 30}`
	var r TurnRecord
	if err := json.Unmarshal([]byte(oldJSON), &r); err != nil {
		t.Fatalf("Failed to unmarshal legacy TurnRecord: %v", err)
	}
	if r.Turn != 1 || r.NewInputTokens != 10 || r.CachedInputTokens != 20 || r.OutputTokens != 30 {
		t.Errorf("Unexpected unmarshaled legacy TurnRecord: %+v", r)
	}
	if r.ReasoningTokens != 0 {
		t.Errorf("ReasoningTokens: got %d, want 0", r.ReasoningTokens)
	}
}
