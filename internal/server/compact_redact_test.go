package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// Issue 1303 M3: compact_summary and custom_instructions are Claude's compaction
// text. The hook drops them before the payload crosses the socket, so they are
// never posted and never spooled. The evidence that the payload is Claude's and
// the key names in the diagnostic still reach the collector.

const (
	redactSummarySentinel      = "REDACT-SUMMARY-SENTINEL-1303M3"
	redactInstructionsSentinel = "REDACT-INSTR-SENTINEL-1303M3"
)

func TestPrepareDropsClaudeCompactTextKeepsEvidence(t *testing.T) {
	isolateCompactEnv(t)
	path := writeTranscript(t, claudeAssistantLineFixture)
	raw := []byte(fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"hook_event_name":"PostCompact","trigger":"manual","compact_summary":%q,"custom_instructions":%q}`,
		fillSession, path, redactSummarySentinel, redactInstructionsSentinel))

	prepared, err := PrepareCodexTelemetry(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{redactSummarySentinel, redactInstructionsSentinel} {
		if bytes.Contains(prepared, []byte(sentinel)) {
			t.Errorf("prepared payload still holds %s", sentinel)
		}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(prepared, &top); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"compact_summary", "custom_instructions"} {
		if _, ok := top[key]; ok {
			t.Errorf("prepared payload kept %s", key)
		}
	}

	var hook struct {
		Context codexEvidence `json:"harnez_context"`
	}
	if err := json.Unmarshal(prepared, &hook); err != nil {
		t.Fatal(err)
	}
	if !hook.Context.Claude {
		t.Error("evidence Claude = false, want true: the summary key was present before it was dropped")
	}
	if hook.Context.Diag == nil || !slices.Contains(hook.Context.Diag.Keys, "compact_summary") || !slices.Contains(hook.Context.Diag.Keys, "custom_instructions") {
		t.Errorf("diagnostic keys %+v, want both key names recorded", hook.Context.Diag)
	}
}

func TestPrepareWithoutClaudeKeysKeepsEvidenceFalse(t *testing.T) {
	isolateCompactEnv(t)
	path := writeTranscript(t, claudeAssistantLineFixture)
	prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(filepath.Clean(path), false))
	if err != nil {
		t.Fatal(err)
	}
	var hook struct {
		Context codexEvidence `json:"harnez_context"`
	}
	if err := json.Unmarshal(prepared, &hook); err != nil {
		t.Fatal(err)
	}
	if hook.Context.Claude {
		t.Error("evidence Claude = true for a payload without compact_summary")
	}
}
