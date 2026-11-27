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
