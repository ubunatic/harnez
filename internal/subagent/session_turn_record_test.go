package subagent

import (
	"encoding/json"
	"strings"
	"testing"
)

// Ticket 1215: a turn record keeps reported reasoning tokens, stores an
// unreported count as unknown (absent), and old session JSON still loads.
func TestNewTurnRecordReasoningTokens(t *testing.T) {
	known := NewTurnRecord(2, &TurnResult{TokensTurn: 150, CachedTokens: 100, OutputTokens: 30, ReasoningTokens: 42, ReasoningTokensKnown: true})
	if known.Turn != 2 || known.NewInputTokens != 50 || known.CachedInputTokens != 100 || known.OutputTokens != 30 {
		t.Fatalf("token fields = %+v", known)
	}
	if known.ReasoningTokens == nil || *known.ReasoningTokens != 42 {
		t.Fatalf("known reasoning tokens = %v, want 42", known.ReasoningTokens)
	}

	zero := NewTurnRecord(1, &TurnResult{ReasoningTokensKnown: true})
	b, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"reasoning_tokens":0`) {
		t.Fatalf("reported zero must be stored: %s", b)
	}

	unknown := NewTurnRecord(1, &TurnResult{ReasoningTokens: 0})
	b, err = json.Marshal(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "reasoning_tokens") {
		t.Fatalf("unknown reasoning tokens must be absent, not 0: %s", b)
	}

	var round TurnRecord
	if err := json.Unmarshal([]byte(`{"turn":1,"new_input_tokens":10,"cached_input_tokens":20,"output_tokens":30,"reasoning_tokens":42}`), &round); err != nil {
		t.Fatal(err)
	}
	if round.ReasoningTokens == nil || *round.ReasoningTokens != 42 {
		t.Fatalf("round trip lost reasoning tokens: %+v", round)
	}

	var legacy TurnRecord
	if err := json.Unmarshal([]byte(`{"turn":1,"new_input_tokens":10,"cached_input_tokens":20,"output_tokens":30}`), &legacy); err != nil {
		t.Fatalf("legacy record: %v", err)
	}
	if legacy.ReasoningTokens != nil || legacy.OutputTokens != 30 {
		t.Fatalf("legacy record = %+v", legacy)
	}
}
