package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"ubunatic.com/harnez/internal/telemetry"
)

func TestCodexTelemetryCarriesClientTranscriptAndContext(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "client.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"session_id":"client-session","token_usage":{"total_tokens":107,"input_tokens":100,"output_tokens":7}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"session_id": "client-session", "hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_use_id": "ctx-call", "transcript_path": transcript})
	prepared, err := PrepareCodexTelemetry(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(transcript); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "telemetry.sqlite")
	if err := ProcessCodexTelemetry(path, prepared); err != nil {
		t.Fatal(err)
	}
	db, err := telemetry.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{SessionID: "client-session"})
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if len(rows) != 1 || rows[0].WorkingDir != wd {
		t.Fatalf("client context: %+v", rows)
	}
	// The transcript's cumulative usage must survive the absent client file.
	snapshots, err := db.LatestTokenSnapshots([]string{"client-session"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots["client-session"].TotalTokens == nil || *snapshots["client-session"].TotalTokens != 107 {
		t.Fatalf("client transcript snapshots: %+v", snapshots)
	}
}
