package decide

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type mockBackend struct {
	fn func(ctx context.Context, req *Request) (*Response, error)
}

func (m *mockBackend) Decide(ctx context.Context, req *Request) (*Response, error) {
	return m.fn(ctx, req)
}

func TestCompactTranscript_Pinning(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: "Initial user goal"},
		{Role: "assistant", Content: "Starting task..."},
		{Role: "tool", ToolName: "read", ToolInput: "large.txt", ToolOutput: strings.Repeat("log data\n", 100)},
		{Role: "user", Content: "Recent user update"},
		{Role: "assistant", Content: "Final answer"},
	}

	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			t.Fatalf("recent entries and initial prompt should be pinned without querying backend")
			return nil, nil
		},
	}

	opts := CompactOptions{
		PinRecent: 4, // Pins initial entry (index 0) + last 4 entries (indices 1, 2, 3, 4) -> all pinned
	}

	out, rep, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if len(out) != len(entries) {
		t.Errorf("got %d entries, want %d", len(out), len(entries))
	}
	if rep.KeptVerbatim != len(entries) {
		t.Errorf("got %d kept verbatim, want %d", rep.KeptVerbatim, len(entries))
	}
}

func TestCompactTranscript_SplitToolCallAndResultPairing(t *testing.T) {
	callTrue := 0.90
	callFalse := 0.10
	resultFalse := 0.10

	entries := []TranscriptEntry{
		{Role: "user", Content: "Initial goal"},
		// Pair 1: Assistant call + Tool result -> Both excised
		{Role: "assistant", Content: "Running check", ToolCallID: "call_check_1", ToolName: "check"},
		{Role: "tool", ToolCallID: "call_check_1", ToolOutput: "check passed"},
		// Pair 2: Assistant call + Tool result -> Keep call, truncate result (standard tool result with content field)
		{Role: "assistant", Content: "Reading big file", ToolCallID: "call_read_2", ToolName: "read"},
		{Role: "tool", ToolCallID: "call_read_2", Content: strings.Repeat("data line\n", 50)},
		// Recent pinned messages
		{Role: "user", Content: "Recent question"},
		{Role: "assistant", Content: "Recent answer"},
	}

	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			if req.Model != "jev-custom" {
				t.Errorf("got req.Model = %q, want jev-custom", req.Model)
			}
			// Verify candidate state includes tool context
			cands, ok := req.State.(map[string]any)["candidates"].(map[string]any)
			if !ok || len(cands) != 2 {
				t.Errorf("expected 2 candidates in state, got: %+v", cands)
			}

			answers := make(map[string]Answer)
			answers["cand_0_keep_call"] = Answer{Type: TypeNoul, Noul: &callFalse}
			answers["cand_0_keep_result"] = Answer{Type: TypeNoul, Noul: &resultFalse}
			answers["cand_1_keep_call"] = Answer{Type: TypeNoul, Noul: &callTrue}
			answers["cand_1_keep_result"] = Answer{Type: TypeNoul, Noul: &resultFalse}
			return &Response{Answers: answers}, nil
		},
	}

	opts := CompactOptions{
		Model:          "jev-custom",
		PinRecent:      2,
		Threshold:      0.50,
		TruncateLength: 40,
	}

	out, rep, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if rep.ExcisedTools != 1 {
		t.Errorf("got %d excised tools, want 1", rep.ExcisedTools)
	}
	if rep.TruncatedResults != 1 {
		t.Errorf("got %d truncated results, want 1", rep.TruncatedResults)
	}

	// Initial (1) + Pair 2 (2) + Recent (2) = 5 entries
	if len(out) != 5 {
		t.Fatalf("got %d entries, want 5: %+v", len(out), out)
	}

	// Check that neither part of pair 1 remains
	for _, e := range out {
		if e.ToolCallID == "call_check_1" {
			t.Errorf("found excised call_check_1 entry in output: %+v", e)
		}
	}

	// Verify truncation length is strictly <= TruncateLength (40)
	for _, e := range out {
		if e.ToolCallID == "call_read_2" && e.Role == "tool" {
			if len([]rune(e.Content)) > 40 {
				t.Errorf("tool content length %d exceeded TruncateLength 40: %q", len([]rune(e.Content)), e.Content)
			}
		}
	}
}

func TestCompactTranscript_ShortTruncateLengthStrictBound(t *testing.T) {
	callTrue := 0.90
	resultFalse := 0.10

	entries := []TranscriptEntry{
		{Role: "user", Content: "Goal"},
		{Role: "tool", ToolName: "log", ToolOutput: "1234567890abcdefghij"},
		{Role: "assistant", Content: "Done"},
	}

	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				Answers: map[string]Answer{
					"cand_0_keep_call":   {Type: TypeNoul, Noul: &callTrue},
					"cand_0_keep_result": {Type: TypeNoul, Noul: &resultFalse},
				},
			}, nil
		},
	}

	// TruncateLength is 10 (shorter than the 34-rune marker)
	opts := CompactOptions{
		PinRecent:      1,
		Threshold:      0.50,
		TruncateLength: 10,
	}

	out, _, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if len(out) != 3 {
		t.Fatalf("got %d entries, want 3", len(out))
	}
	if len([]rune(out[1].ToolOutput)) > 10 {
		t.Errorf("expected output length <= 10, got %d: %q", len([]rune(out[1].ToolOutput)), out[1].ToolOutput)
	}
}

func TestCompactTranscript_ZeroThresholdIsPreserved(t *testing.T) {
	belowDefaultThreshold := 0.10

	entries := []TranscriptEntry{
		{Role: "user", Content: "Goal"},
		{Role: "tool", ToolName: "log", ToolOutput: strings.Repeat("output", 20)},
		{Role: "assistant", Content: "Done"},
	}

	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				Answers: map[string]Answer{
					"cand_0_keep_call":   {Type: TypeNoul, Noul: &belowDefaultThreshold},
					"cand_0_keep_result": {Type: TypeNoul, Noul: &belowDefaultThreshold},
				},
			}, nil
		},
	}

	out, report, err := CompactTranscript(context.Background(), mock, entries, CompactOptions{
		PinRecent:      1,
		Threshold:      0,
		TruncateLength: 10,
	})
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}
	if report.TruncatedResults != 0 || report.ExcisedTools != 0 {
		t.Fatalf("zero threshold must keep both parts, got report: %+v", report)
	}
	if got := out[1].ToolOutput; got != entries[1].ToolOutput {
		t.Errorf("tool output was changed at zero threshold: %q", got)
	}
}

func TestCompactTranscript_PinnedPairProtection(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: "Goal"},
		// Call is index 1 (older than recent threshold 2)
		{Role: "assistant", Content: "Executing tool", ToolCallID: "call_pinned", ToolName: "test"},
		// Result is index 2 (within recent threshold: entries length 3, pinRecent 2 -> recentThreshold is 1)
		{Role: "tool", ToolCallID: "call_pinned", ToolOutput: "important result"},
	}

	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			t.Fatalf("pinned pair should not be queried")
			return nil, nil
		},
	}

	opts := CompactOptions{
		PinRecent: 2,
	}

	out, rep, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if len(out) != len(entries) {
		t.Errorf("got %d entries, want %d", len(out), len(entries))
	}
	if rep.ExcisedTools != 0 || rep.TruncatedResults != 0 {
		t.Errorf("expected zero excised/truncated tools for pinned pair, got report: %+v", rep)
	}
}

func TestParseAndWriteJSONLTranscript_RoundTrip(t *testing.T) {
	jsonl := `{"content":"hello","role":"user"}
{"content":"contents","role":"tool","tool_name":"cat"}
{"content":"done","role":"assistant"}
`
	entries, err := ParseJSONLTranscript(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("ParseJSONLTranscript failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	var buf bytes.Buffer
	if err := WriteJSONLTranscript(&buf, entries); err != nil {
		t.Fatalf("WriteJSONLTranscript failed: %v", err)
	}

	reparsed, err := ParseJSONLTranscript(&buf)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}

	if len(reparsed) != len(entries) {
		t.Errorf("got %d reparsed entries, want %d", len(reparsed), len(entries))
	}
	if reparsed[0].Content != "hello" || reparsed[1].Content != "contents" {
		t.Errorf("reparsed entries do not match: %+v", reparsed)
	}
}

func TestParseJSONLTranscript_LineNumberOnError(t *testing.T) {
	jsonl := `{"role":"user","content":"first"}

{"role":"tool","tool_name":"cat"}

{invalid-json}
`
	_, err := ParseJSONLTranscript(strings.NewReader(jsonl))
	if err == nil {
		t.Fatalf("expected error on invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "line 5") {
		t.Errorf("expected error to reference line 5, got: %v", err)
	}
}

func TestCompactTranscript_ConversationContext(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: "Build a new widget"},
		{Role: "assistant", Content: "I will check the directory"},
		{Role: "tool", ToolName: "ls", ToolOutput: "a\nb\nc\nd\ne\nf"},
		{Role: "assistant", Content: "Found the files"},
	}

	var capturedReq *Request
	callTrue := 0.9
	resultFalse := 0.1
	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			capturedReq = req
			return &Response{
				Answers: map[string]Answer{
					"cand_0_keep_call":   {Type: TypeNoul, Noul: &callTrue},
					"cand_0_keep_result": {Type: TypeNoul, Noul: &resultFalse},
				},
			}, nil
		},
	}

	opts := CompactOptions{
		PinRecent:      1,
		Threshold:      0.5,
		TruncateLength: 10,
	}

	_, _, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatalf("expected decide request to be sent, got nil")
	}

	stateMap, ok := capturedReq.State.(map[string]any)
	if !ok {
		t.Fatalf("expected state to be map[string]any, got %T", capturedReq.State)
	}

	initInst, ok := stateMap["initial_instruction"].(string)
	if !ok || initInst != "Build a new widget" {
		t.Errorf("expected initial_instruction 'Build a new widget', got %v", stateMap["initial_instruction"])
	}

	convCtx, ok := stateMap["conversation_context"].([]map[string]string)
	if !ok || len(convCtx) < 2 {
		t.Errorf("expected conversation_context to contain messages, got %+v", stateMap["conversation_context"])
	}
}

func TestWriteJSONLTranscript_RawMapUpdatedOnTruncation(t *testing.T) {
	jsonl := `{"role":"user","content":"start"}
{"role":"tool","tool_name":"cat","tool_output":"` + strings.Repeat("very long output line ", 30) + `"}
{"role":"assistant","content":"done"}
`
	entries, err := ParseJSONLTranscript(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("ParseJSONLTranscript failed: %v", err)
	}

	callTrue := 0.9
	resultFalse := 0.1
	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				Answers: map[string]Answer{
					"cand_0_keep_call":   {Type: TypeNoul, Noul: &callTrue},
					"cand_0_keep_result": {Type: TypeNoul, Noul: &resultFalse},
				},
			}, nil
		},
	}

	opts := CompactOptions{
		PinRecent:      1,
		Threshold:      0.5,
		TruncateLength: 200,
	}

	compacted, _, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteJSONLTranscript(&buf, compacted); err != nil {
		t.Fatalf("WriteJSONLTranscript failed: %v", err)
	}

	serialized := buf.String()
	if strings.Contains(serialized, strings.Repeat("very long output line ", 30)) {
		t.Errorf("expected serialized raw JSON to NOT contain full untruncated content, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "harnez compact truncated 548 chars") {
		t.Errorf("expected serialized raw JSON to contain exact truncated count '548 chars', got:\n%s", serialized)
	}
}

func TestCompactTranscript_ByteAndReductionMetrics(t *testing.T) {
	jsonl := `{"role":"user","content":"start"}
{"role":"tool","tool_name":"cat","tool_output":"` + strings.Repeat("a", 1000) + `"}
{"role":"assistant","content":"done"}
`
	entries, err := ParseJSONLTranscript(strings.NewReader(jsonl))
	if err != nil {
		t.Fatal(err)
	}

	callTrue := 0.9
	resultFalse := 0.1
	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{
				Answers: map[string]Answer{
					"cand_0_keep_call":   {Type: TypeNoul, Noul: &callTrue},
					"cand_0_keep_result": {Type: TypeNoul, Noul: &resultFalse},
				},
			}, nil
		},
	}

	opts := CompactOptions{
		PinRecent:      1,
		Threshold:      0.5,
		TruncateLength: 50,
	}

	_, rep, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if rep.OriginalBytes <= 0 {
		t.Errorf("expected positive OriginalBytes, got %d", rep.OriginalBytes)
	}
	if rep.CompactedBytes >= rep.OriginalBytes {
		t.Errorf("expected CompactedBytes (%d) < OriginalBytes (%d)", rep.CompactedBytes, rep.OriginalBytes)
	}
	if rep.ReductionRatio <= 0.0 || rep.ReductionRatio > 1.0 {
		t.Errorf("expected valid ReductionRatio (0.0 < r <= 1.0), got %f", rep.ReductionRatio)
	}
}

func TestFitCompactionState_ProgressiveStages(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: "First instruction"},
		{Role: "assistant", Content: strings.Repeat("long explanation ", 50)},
		{Role: "tool", ToolName: "build", ToolInput: strings.Repeat("args ", 100), ToolOutput: "build complete"},
		{Role: "assistant", Content: "Done"},
	}

	candidates := []toolCandidate{
		{
			callIdx:   2,
			resultIdx: 2,
			toolName:  "build",
			toolInput: strings.Repeat("args ", 100),
			outputLen: 14,
			callID:    "t0",
		},
	}

	// Budget 500 bytes should trigger progressive fitting
	state, questions := fitCompactionState(entries, candidates, 500)
	if state == nil || questions == nil {
		t.Fatalf("expected valid state and questions, got nil")
	}

	b, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if len(b) > 500 {
		t.Errorf("expected serialized state <= 500 bytes, got %d", len(b))
	}
}

func TestFitCompactionState_StrictSizeBudget(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: strings.Repeat("massive user goal prompt ", 200)},
		{Role: "assistant", Content: strings.Repeat("massive assistant response ", 200)},
	}

	var candidates []toolCandidate
	for i := 0; i < 20; i++ {
		candidates = append(candidates, toolCandidate{
			callIdx:   i + 2,
			resultIdx: i + 2,
			toolName:  fmt.Sprintf("tool_%d", i),
			toolInput: strings.Repeat("huge args payload ", 50),
			outputLen: 1000,
			callID:    fmt.Sprintf("t%d", i),
		})
	}

	for _, maxBudget := range []int{600, 100, 10, 2} {
		state, _ := fitCompactionState(entries, candidates, maxBudget)
		b, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		if len(b) > maxBudget {
			t.Errorf("expected serialized state <= %d bytes, got %d", maxBudget, len(b))
		}
	}
}

func TestCompactTranscript_QuestionPrompts(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: "Goal"},
		{Role: "tool", ToolName: "grep", ToolInput: "pattern", ToolOutput: "result", ToolCallID: "call_abc"},
		{Role: "assistant", Content: "Done"},
	}

	var capturedReq *Request
	mock := &mockBackend{
		fn: func(ctx context.Context, req *Request) (*Response, error) {
			capturedReq = req
			callTrue := 0.9
			return &Response{
				Answers: map[string]Answer{
					"cand_0_keep_call":   {Type: TypeNoul, Noul: &callTrue},
					"cand_0_keep_result": {Type: TypeNoul, Noul: &callTrue},
				},
			}, nil
		},
	}

	opts := CompactOptions{
		PinRecent: 1,
	}

	_, _, err := CompactTranscript(context.Background(), mock, entries, opts)
	if err != nil {
		t.Fatalf("CompactTranscript failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatalf("expected decide request, got nil")
	}

	qCall, ok := capturedReq.Questions["cand_0_keep_call"]
	if !ok {
		t.Fatalf("missing cand_0_keep_call question")
	}
	wantCall := "Tool call call_abc (grep) should stay in the history: knowing this call was made, with its input, still matters for what the assistant does next"
	if qCall.Instructions != wantCall {
		t.Errorf("want exact keep_call prompt %q, got %q", wantCall, qCall.Instructions)
	}

	qResult, ok := capturedReq.Questions["cand_0_keep_result"]
	if !ok {
		t.Fatalf("missing cand_0_keep_result question")
	}
	wantResult := "The full output of tool call call_abc (grep, 6 bytes) should stay in the history verbatim: the assistant still needs its contents and re-running the tool would not do"
	if qResult.Instructions != wantResult {
		t.Errorf("want exact keep_result prompt %q, got %q", wantResult, qResult.Instructions)
	}
}
