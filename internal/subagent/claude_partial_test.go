package subagent

import (
	"fmt"
	"strings"
	"testing"
)

// partialClaudeCall is one assistant line with a usage block. total is the
// request prompt size: input 2, cache creation 4694, the rest cache read.
func partialClaudeCall(msgID string, total int) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"model":"claude-haiku-5-5","id":%q,"usage":{"input_tokens":2,"cache_creation_input_tokens":4694,"cache_read_input_tokens":%d,"output_tokens":4}},"parent_tool_use_id":null,"session_id":"sess-live"}`, msgID, total-2-4694)
}

// A stopped turn has no result line. Its complete calls still count, and a
// truncated last line (the kill cut it off) is skipped, not an error.
func TestParseClaudePartialCountsCompleteCallsOnly(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-live","model":"claude-haiku-5-5"}`,
		partialClaudeCall("msg_a", 50000),
		partialClaudeCall("msg_b", 60000),
		`{"type":"assistant","message":{"id":"msg_c"`,
	}, "\n")

	r := parseClaudePartial([]byte(data))
	if r == nil {
		t.Fatal("parseClaudePartial returned nil")
	}
	if r.ContextTokens != 60000 || r.PeakContextTokens != 60000 {
		t.Fatalf("ContextTokens=%d PeakContextTokens=%d, want 60000 and 60000", r.ContextTokens, r.PeakContextTokens)
	}
	if r.InputTokens != 4 || r.OutputTokens != 8 {
		t.Fatalf("InputTokens=%d OutputTokens=%d, want 4 and 8", r.InputTokens, r.OutputTokens)
	}
	// CacheRead 45304 + 55304 and CacheWrite 4694 * 2.
	if r.CachedTokens != 109996 {
		t.Fatalf("CachedTokens=%d, want 109996", r.CachedTokens)
	}
	if !r.CacheWriteTokensKnown || r.CacheWriteTokens != 9388 {
		t.Fatalf("CacheWriteTokens=%d known=%v, want 9388 and true", r.CacheWriteTokens, r.CacheWriteTokensKnown)
	}
	if len(r.RequestUsage) != 2 {
		t.Fatalf("RequestUsage has %d calls, want 2", len(r.RequestUsage))
	}
}

// partialClaudeSubagentCall is a sub-agent assistant line: its parent_tool_use_id
// names a tool, so its prompt is not the main chain's context (ticket 1264 M3).
func partialClaudeSubagentCall(msgID string, total int) string {
	return strings.Replace(partialClaudeCall(msgID, total), `"parent_tool_use_id":null`, `"parent_tool_use_id":"toolu_sub"`, 1)
}

// The peak is the largest main-chain prompt, not the last one: a turn that
// shrinks after compaction keeps its high-water mark (item 3).
func TestParseClaudePartialPeakIsMainChainMax(t *testing.T) {
	data := strings.Join([]string{
		partialClaudeCall("msg_a", 90000),
		partialClaudeCall("msg_b", 60000),
	}, "\n")
	r := parseClaudePartial([]byte(data))
	if r.PeakContextTokens != 90000 || r.ContextTokens != 60000 {
		t.Fatalf("PeakContextTokens=%d ContextTokens=%d, want 90000 and 60000", r.PeakContextTokens, r.ContextTokens)
	}
}

// Sub-agent prompts never set the main-chain peak or context (item 3).
func TestParseClaudePartialExcludesSubagentFromPeak(t *testing.T) {
	data := strings.Join([]string{
		partialClaudeCall("msg_a", 90000),
		partialClaudeSubagentCall("msg_sub", 150000),
		partialClaudeCall("msg_b", 60000),
	}, "\n")
	r := parseClaudePartial([]byte(data))
	if r.PeakContextTokens != 90000 || r.ContextTokens != 60000 {
		t.Fatalf("PeakContextTokens=%d ContextTokens=%d, want 90000 and 60000 (sub-agent excluded)", r.PeakContextTokens, r.ContextTokens)
	}
}

// A completed turn reports the same main-chain peak through the strict parser (item 3).
func TestParseClaudeCompletedTurnPeak(t *testing.T) {
	result := `{"type":"result","subtype":"success","is_error":false,"session_id":"sess-live","result":"ok","usage":{"input_tokens":6,"cache_creation_input_tokens":4694,"cache_read_input_tokens":115300,"output_tokens":12}}`
	data := strings.Join([]string{
		partialClaudeCall("msg_a", 90000),
		partialClaudeCall("msg_b", 60000),
		result,
	}, "\n") + "\n"
	r, err := parseClaude([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if r.PeakContextTokens != 90000 || r.ContextTokens != 60000 {
		t.Fatalf("PeakContextTokens=%d ContextTokens=%d, want 90000 and 60000", r.PeakContextTokens, r.ContextTokens)
	}
}

// One message id that streams in several lines with growing usage counts once,
// at its last complete usage; a cut-off repeat of the id adds nothing (item 4).
func TestParseClaudePartialRepeatedIDCountsOnceAtLastUsage(t *testing.T) {
	data := strings.Join([]string{
		partialClaudeCall("msg_a", 50000),
		partialClaudeCall("msg_a", 70000),
		`{"type":"assistant","message":{"id":"msg_a"`,
	}, "\n")
	r := parseClaudePartial([]byte(data))
	if len(r.RequestUsage) != 1 {
		t.Fatalf("RequestUsage has %d calls, want 1 (one message id)", len(r.RequestUsage))
	}
	if r.ContextTokens != 70000 || r.PeakContextTokens != 70000 {
		t.Fatalf("ContextTokens=%d PeakContextTokens=%d, want 70000 and 70000", r.ContextTokens, r.PeakContextTokens)
	}
	if r.InputTokens != 2 || r.OutputTokens != 4 {
		t.Fatalf("InputTokens=%d OutputTokens=%d, want 2 and 4 (last usage only)", r.InputTokens, r.OutputTokens)
	}
	// CacheRead 65304 + CacheWrite 4694.
	if r.CachedTokens != 69998 {
		t.Fatalf("CachedTokens=%d, want 69998", r.CachedTokens)
	}
}
