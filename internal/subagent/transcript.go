package subagent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"time"
)

// ClaudeTranscriptCall is one Claude API call read from a session transcript
// (~/.claude/projects/<dir>/<id>.jsonl). Usage.Model is the call's own
// message.model, so the call is priced by the model that served it (ticket 1302 M1).
type ClaudeTranscriptCall struct {
	MessageID string
	At        time.Time
	Usage     RequestUsage
}

// claudeTranscriptLine is the part of one transcript line the report needs.
type claudeTranscriptLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID    string       `json:"id"`
		Model string       `json:"model"`
		Usage *claudeUsage `json:"usage"`
	} `json:"message"`
}

// ParseClaudeTranscriptCalls returns one call per message id of a transcript.
// A streamed call repeats its usage on several assistant lines and the last
// one carries the final counts; output tokens only grow while streaming, so the
// line with the most output wins (the same final usage parseClaude keeps).
// Lines without usage, all-zero usage, or without a message id are skipped;
// a call with no timestamp keeps a zero At and is never attributed. A line over
// 16 MiB is an error: the calls after it would be silently lost.
func ParseClaudeTranscriptCalls(data []byte) ([]ClaudeTranscriptCall, error) {
	var calls []ClaudeTranscriptCall
	index := map[string]int{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e claudeTranscriptLine
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		u := e.Message.Usage
		if e.Type != "assistant" || u == nil || e.Message.ID == "" || u.Input+u.CacheRead+u.CacheCreate+u.Output == 0 {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
		call := ClaudeTranscriptCall{MessageID: e.Message.ID, At: at, Usage: u.requestUsage(e.Message.Model)}
		if i, seen := index[call.MessageID]; seen {
			if call.Usage.Output >= calls[i].Usage.Output {
				calls[i] = call
			}
			continue
		}
		index[call.MessageID] = len(calls)
		calls = append(calls, call)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return calls, nil
}
