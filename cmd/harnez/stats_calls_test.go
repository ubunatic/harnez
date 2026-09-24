package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/telemetry"
)

func TestRunStatsCallsMergesToolsAndTotalsPrompt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tool_catalog.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	rows := []telemetry.ToolCall{
		{CreatedAt: base, SessionID: "agent-1", AgentID: "agy", ToolName: "run_command", CallType: "hook:prep", Note: "preps:Bash | agy-route=exec | git status"},
		{CreatedAt: base.Add(time.Second), SessionID: "agent-1", AgentID: "agy", ToolName: "Bash", CallType: "shell", ActualTokens: int64Ptr(8)},
		{CreatedAt: base.Add(2 * time.Second), SessionID: "agent-1", AgentID: "agy", ToolName: "Read", CallType: "hook:rpc"},
		{CreatedAt: base.Add(time.Second), SessionID: "other", AgentID: "agy", ToolName: "Ignored", CallType: "shell"},
	}
	for _, row := range rows {
		if err := db.Insert(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	meterPath := filepath.Join(dir, "usage.jsonl")
	meterRows := []agymeter.Record{
		{Time: base.Add(500 * time.Millisecond), Kind: "usage", Session: "agent-1", PromptID: "turn-1", Model: "gemini-flash", Prompt: 100, Cached: 20, Candidates: 4, Thoughts: 2, Total: 106},
		{Time: base.Add(3 * time.Second), Kind: "usage", Session: "agent-1", PromptID: "turn-1", Model: "gemini-flash", Prompt: 10, Candidates: 1, Total: 11},
		{Time: base, Kind: "usage", Session: "other", PromptID: "other-turn", Model: "ignored", Prompt: 999},
	}
	f, err := os.Create(meterPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range meterRows {
		b, _ := json.Marshal(row)
		_, _ = f.Write(append(b, '\n'))
	}
	_ = f.Close()
	var out bytes.Buffer
	if err := runStats(&out, statsOptions{DBPath: dbPath, MeterPath: meterPath, Session: "agent-1", Calls: true, JSON: true}); err != nil {
		t.Fatal(err)
	}
	var report statsCallsReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Session != "agent-1" || len(report.Rows) != 4 || len(report.Prompts) != 1 {
		t.Fatalf("report: %+v", report)
	}
	if report.Prompts[0].Requests != 2 || report.Prompts[0].PromptTokens != 110 || report.Prompts[0].TotalTokens != 117 || report.Prompts[0].ToolCalls != 2 {
		t.Fatalf("prompt total: %+v", report.Prompts[0])
	}
	if report.Prompts[0].Label == "" || report.Prompts[0].Label == report.Prompts[0].PromptID || !strings.HasPrefix(report.Prompts[0].Label, "Prompt 1") {
		t.Fatalf("prompt label = %q, want a readable index/time label", report.Prompts[0].Label)
	}
	merged := false
	for _, row := range report.Rows {
		if row.Sources == "hook+exec" {
			merged = true
		}
		if row.Tool == "run_command" {
			t.Fatal("duplicate hook prep row was retained")
		}
		if row.Model == "ignored" {
			t.Fatal("other session meter row leaked")
		}
	}
	if !merged {
		t.Fatal("hook and exec rows were not merged")
	}
	var table bytes.Buffer
	if err := runStats(&table, statsOptions{DBPath: dbPath, MeterPath: meterPath, Session: "agent-1", Calls: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(table.String(), "%!(EXTRA") || !strings.Contains(table.String(), "hook+exec") || !strings.Contains(table.String(), "Prompt 1 ·") {
		t.Fatalf("table output is malformed or missing merged source:\n%s", table.String())
	}
}

func TestRunStatsCallsRequiresSession(t *testing.T) {
	err := runStats(&bytes.Buffer{}, statsOptions{Calls: true, DBPath: filepath.Join(t.TempDir(), "db.sqlite")})
	if err == nil || !strings.Contains(err.Error(), "requires --session") {
		t.Fatalf("error: %v", err)
	}
}
