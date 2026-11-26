package subagent

import "testing"

func TestParseClaudeKeepsOnePerAPICallWithCacheSplit(t *testing.T) {
	stream := `{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_a","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":100,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":30}}}}
{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_a","usage":{"input_tokens":1,"output_tokens":9,"cache_read_input_tokens":100,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":30}}}}
{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"id":"msg_sub","usage":{"input_tokens":2,"output_tokens":3,"cache_read_input_tokens":0,"cache_creation_input_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":5,"ephemeral_1h_input_tokens":0}}}}
{"type":"assistant","parent_tool_use_id":null,"message":{"id":"msg_nosplit","usage":{"input_tokens":4,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":7}}}
{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"s1","usage":{"input_tokens":7,"output_tokens":13,"cache_read_input_tokens":100,"cache_creation_input_tokens":42}}
`
	r, err := parseClaude([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.RequestUsage) != 3 {
		t.Fatalf("RequestUsage = %+v; want 3 API calls (msg_a once, sub-agent call included)", r.RequestUsage)
	}
	first := r.RequestUsage[0]
	if first.Output != 9 || first.CacheRead != 100 || first.CacheWrite1h != 30 || first.CacheWrite5m != 0 || !first.CacheWriteSplitKnown {
		t.Fatalf("msg_a = %+v; want last streamed usage (output 9) with the 1h write split", first)
	}
	if r.RequestUsage[1].CacheWrite5m != 5 || r.RequestUsage[1].Input != 2 {
		t.Fatalf("sub-agent call = %+v; want 5m write 5, input 2", r.RequestUsage[1])
	}
	if r.RequestUsage[2].CacheWriteSplitKnown {
		t.Fatalf("call without a cache_creation split = %+v; want CacheWriteSplitKnown=false so its cost stays unknown", r.RequestUsage[2])
	}
	var in, read, cw, out int
	for _, q := range r.RequestUsage {
		in += q.Input
		read += q.CacheRead
		cw += q.CacheWriteTotal
		out += q.Output
	}
	if in != r.InputTokens || read != r.CachedTokens-r.CacheWriteTokens || cw != r.CacheWriteTokens || out != r.OutputTokens {
		t.Fatalf("per-call sums in=%d read=%d write=%d out=%d disagree with turn totals in=%d cached=%d write=%d out=%d",
			in, read, cw, out, r.InputTokens, r.CachedTokens, r.CacheWriteTokens, r.OutputTokens)
	}
}
