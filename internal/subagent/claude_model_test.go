package subagent

import "testing"

// Each API call keeps the model id from its own message, including sub-agent
// calls, so a Haiku sub-agent under an Opus session is priced as Haiku (ticket 1301 M3).
func TestParseClaudeRecordsEachCallsModel(t *testing.T) {
	stream := `{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_h","model":"claude-haiku-5-5","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"id":"msg_o","model":"claude-opus-5-5-20261101","usage":{"input_tokens":2,"output_tokens":3,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"s1","usage":{"input_tokens":3,"output_tokens":4}}
`
	r, err := parseClaude([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.RequestUsage) != 2 {
		t.Fatalf("RequestUsage = %+v; want 2 API calls", r.RequestUsage)
	}
	if got := r.RequestUsage[0].Model; got != "claude-haiku-5-5" {
		t.Fatalf("main call model = %q; want claude-haiku-5-5 from its own message", got)
	}
	if got := r.RequestUsage[1].Model; got != "claude-opus-5-5-20261101" {
		t.Fatalf("sub-agent call model = %q; want the id from its own message, unchanged", got)
	}
	// The turn's model is the main chain's, so a sub-agent cannot relabel the rows.
	if r.Model != "claude-haiku-5-5" {
		t.Fatalf("turn model = %q; want the main-chain call's claude-haiku-5-5", r.Model)
	}
}
