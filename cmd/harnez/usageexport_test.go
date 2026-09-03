package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

// TestRunUsageExport_EndToEndScrubsRawPII builds a temp "~/.harnez"-style
// fixture (a telemetry sqlite db plus a usage-history jsonl file), each
// seeded with a real-looking absolute path, email, and hostname, runs
// `harnez usage export` against them end-to-end, and asserts by string
// search over the written output file that none of the raw values survive.
func TestRunUsageExport_EndToEndScrubsRawPII(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tool_catalog.sqlite")
	historyDir := filepath.Join(dir, "usage-history")
	outPath := filepath.Join(dir, "export.json")

	rawPath := "/home/testuser/projects/foo"
	rawEmail := "someone@example.com"
	rawHost := "testusers-workstation.local"

	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	if err := db.Insert(telemetry.ToolCall{
		SessionID:   "sess-1",
		TicketID:    "204",
		ProjectName: "foo",
		WorkingDir:  rawPath,
		AgentID:     "claude",
		ToolName:    "Read",
		CallType:    "internal",
		RawBytes:    10,
	}); err != nil {
		db.Close()
		t.Fatalf("Insert: %v", err)
	}
	db.Close()

	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		t.Fatalf("mkdir history dir: %v", err)
	}
	entry := usage.HistoryEntry{
		Hostname: rawHost,
		UsageSummary: usage.UsageSummary{
			Timestamp: time.Now(),
			Agents: []usage.AgentUsage{
				{AgentID: "claude", Account: rawEmail, Tokens: &usage.TokenBreakdown{TotalTokens: 500}},
			},
		},
	}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal history fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(historyDir, "host1.jsonl"), append(line, '\n'), 0o600); err != nil {
		t.Fatalf("write history fixture: %v", err)
	}

	if err := runUsageExport(outPath, dbPath, historyDir); err != nil {
		t.Fatalf("runUsageExport: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read export output: %v", err)
	}
	out := string(data)

	for _, raw := range []string{rawPath, "/home/testuser", "testuser", rawEmail, "someone", rawHost} {
		if strings.Contains(out, raw) {
			t.Errorf("raw sensitive value %q leaked into export output:\n%s", raw, out)
		}
	}

	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("unmarshal export output: %v", err)
	}
	tel, ok := envelope["telemetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected telemetry key in export output, got: %s", out)
	}
	calls, ok := tel["tool_calls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("expected 1 tool_calls entry, got: %v", tel["tool_calls"])
	}
	call := calls[0].(map[string]any)
	if call["project_dir"] != "foo" {
		t.Errorf("expected project_dir %q, got %v", "foo", call["project_dir"])
	}
	if call["activity_category"] != "inspection" {
		t.Errorf("expected activity_category %q, got %v", "inspection", call["activity_category"])
	}

	usg, ok := envelope["usage"].(map[string]any)
	if !ok {
		t.Fatalf("expected usage key in export output, got: %s", out)
	}
	points, ok := usg["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("expected 1 usage point, got: %v", usg["points"])
	}
}

// TestRunUsageExport_RequiresOut asserts the CLI-level guard for a missing
// --out value (checked in newUsageExportCmd's RunE, exercised here via
// runUsageExport directly with an empty path writing nowhere useful is
// instead covered by the cobra-level check; this test targets the
// lower-level function's behavior when the caller does supply a path but
// the fixture directories don't exist yet, which must still succeed since
// telemetry.Open and usage.ExportHistory both tolerate a missing dir/file).
func TestRunUsageExport_ToleratesMissingFixtureDirs(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "does-not-exist", "tool_catalog.sqlite")
	historyDir := filepath.Join(dir, "no-history-here")
	outPath := filepath.Join(dir, "export.json")

	if err := runUsageExport(outPath, dbPath, historyDir); err != nil {
		t.Fatalf("runUsageExport with missing fixture dirs: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read export output: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("unmarshal export output: %v", err)
	}
}
