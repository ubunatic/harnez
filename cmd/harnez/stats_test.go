package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

// seedStatsFixture opens a fresh DB at dbPath and inserts a small, hand-
// computable fixture:
//
//	tool=Read  agent=claude  score=5 exit=0  raw=1000 distilled=200
//	tool=Read  agent=claude  score=1 exit=1  raw=500  distilled=nil
//	tool=Edit  agent=codex   score=4 exit=0  raw=2000 distilled=1000
//
// Expected per-tool:
//
//	Read: 2 calls, avg score (5+1)/2=3.00, failure rate 1/2=50.0% (score<=2 OR exit!=0)
//	Edit: 1 call,  avg score 4.00,          failure rate 0/1=0.0%
//
// Expected per-agent:
//
//	claude: 2 calls, avg score 3.00, failure rate 50.0%
//	codex:  1 call,  avg score 4.00, failure rate 0.0%
//
// Expected distillation savings (only the 2 rows with non-NULL
// distilled_bytes): raw=3000, distilled=1200, ratio=1-(1200/3000)=0.60.
func seedStatsFixture(t *testing.T, dbPath string) {
	t.Helper()
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	rows := []telemetry.ToolCall{
		{
			SessionID: "sess-1", TicketID: "harnez/120", AgentID: "claude",
			ToolName: "Read", CallType: "internal",
			Score: intPtr(5), ExitCode: intPtr(0),
			RawBytes: 1000, DistilledBytes: int64Ptr(200),
		},
		{
			SessionID: "sess-1", TicketID: "harnez/120", AgentID: "claude",
			ToolName: "Read", CallType: "shell",
			Score: intPtr(1), ExitCode: intPtr(1),
			RawBytes: 500,
		},
		{
			SessionID: "sess-2", TicketID: "harnez/120", AgentID: "codex",
			ToolName: "Edit", CallType: "internal",
			Score: intPtr(4), ExitCode: intPtr(0),
			RawBytes: 2000, DistilledBytes: int64Ptr(1000),
		},
	}
	for _, r := range rows {
		if err := db.Insert(r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
}

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

func TestRunStatsTable_MatchesHandComputedFixture(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Read", "2", "3.00", "50.0%",
		"Edit", "1", "4.00", "0.0%",
		"claude",
		"codex",
		"60.00%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

func TestRunStatsJSON_ValidAndMatchesTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, JSON: true}); err != nil {
		t.Fatalf("runStats: %v", err)
	}

	var report statsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, buf.String())
	}
	if report.Empty {
		t.Fatal("report.Empty = true, want false")
	}

	byTool := map[string]telemetry.GroupStats{}
	for _, g := range report.ByTool {
		byTool[g.Key] = g
	}
	read, ok := byTool["Read"]
	if !ok {
		t.Fatal("no Read group in JSON output")
	}
	if read.Count != 2 {
		t.Errorf("Read.Count = %d, want 2", read.Count)
	}
	if diff := read.AvgScore - 3.0; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("Read.AvgScore = %v, want 3.0 (raw float, not string-formatted)", read.AvgScore)
	}
	if read.FailureCount != 1 {
		t.Errorf("Read.FailureCount = %d, want 1", read.FailureCount)
	}

	if diff := report.Savings.Ratio - 0.6; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("Savings.Ratio = %v, want 0.6", report.Savings.Ratio)
	}
	if report.Savings.RawBytes != 3000 || report.Savings.DistilledBytes != 1200 {
		t.Errorf("Savings = %+v, want RawBytes=3000 DistilledBytes=1200", report.Savings)
	}

	// Cross-check against the table renderer: same underlying report,
	// same numbers, just different presentation (issue 120's "round-trips
	// the same numbers as the table view" acceptance criterion).
	var tableBuf bytes.Buffer
	if err := renderStatsTable(&tableBuf, report); err != nil {
		t.Fatalf("renderStatsTable: %v", err)
	}
	if !strings.Contains(tableBuf.String(), "3.00") {
		t.Errorf("table rendering of the JSON-derived report missing formatted avg score 3.00; got:\n%s", tableBuf.String())
	}
}

func TestRunStatsFilters_ToolAgentTicket(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, Tool: "Edit"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Read") {
		t.Errorf("--tool Edit output should not mention Read; got:\n%s", out)
	}
	if !strings.Contains(out, "Edit") {
		t.Errorf("--tool Edit output missing Edit; got:\n%s", out)
	}
}

// TestRunStatsAuto_FiltersToResolvedCurrentSession confirms --auto resolves
// session_id the same way harnez rate/harnez exec do (internal/resolve)
// and filters the report to just that session's rows, excluding the
// fixture's other session entirely.
func TestRunStatsAuto_FiltersToResolvedCurrentSession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	err := runStats(&buf, statsOptions{
		DBPath: dbPath,
		Auto:   true,
		Getenv: func(k string) string {
			if k == "CLAUDE_CODE_SESSION_ID" {
				return "sess-1"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Read") {
		t.Errorf("--auto for sess-1 output missing Read; got:\n%s", out)
	}
	if strings.Contains(out, "Edit") {
		t.Errorf("--auto for sess-1 output should not mention Edit (belongs to sess-2); got:\n%s", out)
	}
	if strings.Contains(out, "codex") {
		t.Errorf("--auto for sess-1 output should not mention codex (belongs to sess-2); got:\n%s", out)
	}
}

func TestRunStatsEmptyResult_TableAndJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, Tool: "NoSuchTool"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	if !strings.Contains(strings.ToLower(out), "no data") {
		t.Errorf("empty-result table output should say 'no data'; got:\n%s", out)
	}

	var jsonBuf bytes.Buffer
	if err := runStats(&jsonBuf, statsOptions{DBPath: dbPath, Tool: "NoSuchTool", JSON: true}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(jsonBuf.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, jsonBuf.String())
	}
	if !report.Empty {
		t.Errorf("report.Empty = false, want true; report: %+v", report)
	}
	if len(report.ByTool) != 0 || len(report.ByAgent) != 0 {
		t.Errorf("expected no by_tool/by_agent groups on empty result, got %+v", report)
	}
}

func TestRunStatsEmptyDB_NoCrash(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	// Open (creates schema) but insert nothing.
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.Close()

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath}); err != nil {
		t.Fatalf("runStats on empty DB: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "no data") {
		t.Errorf("empty DB output should say 'no data'; got:\n%s", buf.String())
	}
}

func TestStatsCmdHelp_DocumentsFlags(t *testing.T) {
	cmd := newStatsCmd()
	cmd.SetArgs([]string{"--help"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--help: %v", err)
	}
	out := buf.String()
	for _, flag := range []string{"--tool", "--agent", "--ticket", "--json"} {
		if !strings.Contains(out, flag) {
			t.Errorf("--help output missing %q; got:\n%s", flag, out)
		}
	}
}
