package subagent

import "testing"

// multiStepStream mimics a real stream-json turn: four model calls whose usage
// sums to 66443 input tokens while the last call's context is only 16854.
const multiStepStream = `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"assistant","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":11577,"cache_creation_input_tokens":4747,"output_tokens":1}}}
{"type":"assistant","message":{"usage":{"input_tokens":8,"cache_read_input_tokens":16324,"cache_creation_input_tokens":216,"output_tokens":1}}}
{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"usage":{"input_tokens":900000,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"output_tokens":1}}}
{"type":"assistant","message":{"usage":{"input_tokens":8,"cache_read_input_tokens":16699,"cache_creation_input_tokens":147,"output_tokens":1}}}
{"type":"result","subtype":"success","session_id":"s1","result":"done","usage":{"input_tokens":34,"cache_read_input_tokens":61140,"cache_creation_input_tokens":5269,"output_tokens":393}}
`

func TestParseClaudeContextIsLastCallNotTurnTotal(t *testing.T) {
	r, err := parseClaude([]byte(multiStepStream))
	if err != nil {
		t.Fatal(err)
	}
	if r.ContextTokens != 16854 {
		t.Fatalf("ContextTokens = %d, want 16854 (last call)", r.ContextTokens)
	}
	if r.TokensTurn != 34+393+61140+5269 {
		t.Fatalf("TokensTurn = %d, want the turn total", r.TokensTurn)
	}
	compacted := false
	got, err := EnsureContextUnderThreshold(r.ContextTokens, 200000, func() (*TurnResult, error) {
		compacted = true
		return nil, nil
	})
	if err != nil || got || compacted {
		t.Fatalf("multi-step turn under threshold must not compact: got=%v compacted=%v err=%v", got, compacted, err)
	}
	if r.TokensTurn < 60000 {
		t.Fatalf("test needs a turn total above the threshold below")
	}
	if _, err := EnsureContextUnderThreshold(r.TokensTurn, 60000, func() (*TurnResult, error) { return nil, nil }); err == nil {
		t.Fatal("sanity: the turn total would have triggered a failing compaction")
	}
}

func TestParseClaudeMissingUsageIsUnknown(t *testing.T) {
	for name, in := range map[string]string{
		"single json": `{"session_id":"s1","result":"done","usage":{"input_tokens":10,"output_tokens":4}}`,
		"stream":      `{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","session_id":"s1","result":"done","usage":{"input_tokens":10}}`,
	} {
		r, err := parseClaude([]byte(in))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.ContextTokens != -1 {
			t.Fatalf("%s: ContextTokens = %d, want -1 (unknown)", name, r.ContextTokens)
		}
		if got, err := EnsureContextUnderThreshold(r.ContextTokens, 1, nil); err != nil || got {
			t.Fatalf("%s: unknown context must skip compaction, got=%v err=%v", name, got, err)
		}
	}
}

func TestParseClaudeCompactTurnUsesBoundaryPostTokens(t *testing.T) {
	in := `{"type":"system","subtype":"compact_boundary","compact_metadata":{"trigger":"manual","pre_tokens":16885,"post_tokens":2092}}
{"type":"user","message":{"content":"<local-command-stdout>Compacted </local-command-stdout>"}}
{"type":"result","subtype":"success","session_id":"s1","result":"Compacted","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}
`
	r, err := parseClaude([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if r.ContextTokens != 2092 {
		t.Fatalf("ContextTokens = %d, want 2092", r.ContextTokens)
	}
	if err := VerifyCompaction(16885, r); err != nil {
		t.Fatalf("real compact turn must verify: %v", err)
	}
}

// Trimmed from a real /compact stream (issue 673): after the boundary Claude
// emits two synthetic assistant events whose usage is all zero.
func TestParseClaudeCompactIgnoresZeroUsageAssistantEvents(t *testing.T) {
	in := `{"type":"system","subtype":"compact_boundary","compact_metadata":{"trigger":"manual","pre_tokens":44835,"post_tokens":4033}}
{"type":"assistant","message":{"model":"<synthetic>","role":"assistant","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"parent_tool_use_id":null}
{"type":"assistant","message":{"model":"<synthetic>","role":"assistant","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"parent_tool_use_id":null}
{"type":"user","message":{"role":"user","content":"<local-command-stdout>Compacted </local-command-stdout>"}}
{"type":"result","subtype":"success","session_id":"s1","result":"","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}
`
	r, err := parseClaude([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if r.ContextTokens != 4033 {
		t.Fatalf("ContextTokens = %d, want 4033", r.ContextTokens)
	}
}

func TestParseClaudeErrorResult(t *testing.T) {
	if _, err := parseClaude([]byte(`{"type":"result","is_error":true,"result":"boom"}`)); err == nil {
		t.Fatal("want error")
	}
}
