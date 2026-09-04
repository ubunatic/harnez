package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
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
// Expected per-project:
//
//	harnez: 2 calls, avg score 3.00, failure rate 50.0%
//	voxi:   1 call,  avg score 4.00, failure rate 0.0%
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
			ProjectName: "harnez",
			ToolName:    "Read", CallType: "internal",
			Score: intPtr(5), ExitCode: intPtr(0),
			RawBytes: 1000, DistilledBytes: int64Ptr(200),
		},
		{
			SessionID: "sess-1", TicketID: "harnez/120", AgentID: "claude",
			ProjectName: "harnez",
			ToolName:    "Read", CallType: "shell",
			Score: intPtr(1), ExitCode: intPtr(1),
			RawBytes: 500,
		},
		{
			SessionID: "sess-2", TicketID: "harnez/120", AgentID: "codex",
			ProjectName: "voxi",
			ToolName:    "Edit", CallType: "internal",
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
		"harnez",
		"voxi",
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

	byProject := map[string]telemetry.GroupStats{}
	for _, g := range report.ByProject {
		byProject[g.Key] = g
	}
	harnezProj, ok := byProject["harnez"]
	if !ok {
		t.Fatal("no harnez group in JSON output's by_project")
	}
	if harnezProj.Count != 2 {
		t.Errorf("harnez.Count = %d, want 2", harnezProj.Count)
	}
	if diff := harnezProj.AvgScore - 3.0; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("harnez.AvgScore = %v, want 3.0", harnezProj.AvgScore)
	}
	if voxi, ok := byProject["voxi"]; !ok || voxi.Count != 1 {
		t.Errorf("voxi group = %+v (ok=%v), want Count=1", voxi, ok)
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

// TestRunStatsFilters_Project covers issue 227's --project flag, and its
// AND-combination with --tool: --project harnez alone should still see
// both harnez rows (Read x2) but not voxi's Edit row; --project harnez
// --tool Edit combined should see neither project's Edit-less rows,
// yielding an empty result.
func TestRunStatsFilters_Project(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, Project: "harnez"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "voxi") {
		t.Errorf("--project harnez output should not mention voxi; got:\n%s", out)
	}
	if !strings.Contains(out, "harnez") {
		t.Errorf("--project harnez output missing harnez; got:\n%s", out)
	}
	if !strings.Contains(out, "Read") {
		t.Errorf("--project harnez output should still show tool Read; got:\n%s", out)
	}

	// AND-combination with --tool: harnez has no Edit rows, so this must
	// be empty, not a crash or a stale non-empty result.
	var combinedBuf bytes.Buffer
	if err := runStats(&combinedBuf, statsOptions{DBPath: dbPath, Project: "harnez", Tool: "Edit"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	if !strings.Contains(strings.ToLower(combinedBuf.String()), "no data") {
		t.Errorf("--project harnez --tool Edit should yield no data; got:\n%s", combinedBuf.String())
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

// overheadFixtureConfig returns a minimal claude.Config with one
// rate_feedback-gated section (40 bytes) and one gated skill (10 bytes),
// so buildRateOverheadReport's claude.ToolFeedbackProtocolBytes has a
// hand-computable expected value (50) instead of depending on the real
// embedded config.yaml's current wording.
func overheadFixtureConfig() *claude.Config {
	return &claude.Config{
		AgentsMD: claude.AgentsMD{
			Global: claude.AgentsMDTarget{
				Sections: []claude.MDSection{
					{Name: "Tool Feedback Protocol", RateFeedback: true, Content: strings.Repeat("x", 40)},
					{Name: "Unrelated Section", Content: strings.Repeat("y", 999)},
				},
			},
		},
		Skills: []claude.Command{
			{Name: "tool-feedback-protocol", RateFeedback: true, Content: strings.Repeat("z", 10)},
			{Name: "unrelated-skill", Content: strings.Repeat("w", 999)},
		},
	}
}

// TestRunStatsOverhead_MatchesHandComputedFixture verifies issue 142's
// --overhead report: real measured call_type="internal" (harnez rate)
// count/bytes from telemetry (2 calls, 1000+2000=3000 bytes, avg 1500 —
// per seedStatsFixture's doc comment) paired with the fixture config's
// instruction-text byte size (40+10=50), and that the token figures are
// clearly derived via the documented ~4-bytes-per-token estimate rather
// than presented as exact/provider-reported.
func TestRunStatsOverhead_MatchesHandComputedFixture(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)
	cfg := overheadFixtureConfig()

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, Overhead: true, Config: cfg}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"harnez rate feedback overhead", "calls: 2", "total call bytes: 3000",
		"instruction text: 50 bytes", "ESTIMATE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--overhead table output missing %q; got:\n%s", want, out)
		}
	}

	var jsonBuf bytes.Buffer
	if err := runStats(&jsonBuf, statsOptions{DBPath: dbPath, JSON: true, Overhead: true, Config: cfg}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(jsonBuf.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, jsonBuf.String())
	}
	if report.Overhead == nil {
		t.Fatal("report.Overhead is nil, want populated")
	}
	o := report.Overhead
	if o.Calls != 2 {
		t.Errorf("Calls = %d, want 2", o.Calls)
	}
	if o.TotalCallBytes != 3000 {
		t.Errorf("TotalCallBytes = %d, want 3000", o.TotalCallBytes)
	}
	if o.AvgCallBytes != 1500 {
		t.Errorf("AvgCallBytes = %v, want 1500", o.AvgCallBytes)
	}
	if o.InstructionBytes != 50 {
		t.Errorf("InstructionBytes = %d, want 50", o.InstructionBytes)
	}
	if o.EstimatedCallTokens != 3000/4 {
		t.Errorf("EstimatedCallTokens = %d, want %d", o.EstimatedCallTokens, 3000/4)
	}
	if o.EstimatedInstructionTokens != 50/4 {
		t.Errorf("EstimatedInstructionTokens = %d, want %d", o.EstimatedInstructionTokens, 50/4)
	}
	if !strings.Contains(strings.ToUpper(o.EstimateMethod), "ESTIMATE") {
		t.Errorf("EstimateMethod = %q, want it to clearly say ESTIMATE", o.EstimateMethod)
	}
}

// TestRunStatsWithoutOverheadFlag_OmitsOverheadField confirms --overhead
// is opt-in: report.Overhead stays nil (and the JSON key absent via
// omitempty) when the flag isn't passed, so existing `harnez stats`
// callers/scripts see no shape change.
func TestRunStatsWithoutOverheadFlag_OmitsOverheadField(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, JSON: true}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	if strings.Contains(buf.String(), "rate_feedback_overhead") {
		t.Errorf("expected no rate_feedback_overhead key without --overhead, got:\n%s", buf.String())
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
	if len(report.ByTool) != 0 || len(report.ByAgent) != 0 || len(report.ByProject) != 0 {
		t.Errorf("expected no by_tool/by_agent/by_project groups on empty result, got %+v", report)
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

// Issue 179: `harnez stats` surfaces a session's heartbeat (`harnez rate
// --ok`) history via HeartbeatStats — last-heartbeat time and how many
// tool_calls rows (any call_type) landed since.
func TestRunStats_ReportsHeartbeatInfo(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool_catalog.sqlite")
	seedStatsFixture(t, dbPath)

	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Insert(telemetry.ToolCall{
		SessionID: "sess-1", TicketID: "harnez/179", AgentID: "claude",
		ToolName: "heartbeat", CallType: telemetry.HeartbeatCallType,
		Note: "ok", RawBytes: 20,
	}); err != nil {
		t.Fatalf("Insert heartbeat: %v", err)
	}
	db.Close()

	var buf bytes.Buffer
	if err := runStats(&buf, statsOptions{DBPath: dbPath, JSON: true}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, buf.String())
	}
	if report.Heartbeat.Count != 1 {
		t.Errorf("Heartbeat.Count = %d, want 1", report.Heartbeat.Count)
	}
	if report.Heartbeat.LastAt.IsZero() {
		t.Error("Heartbeat.LastAt is zero, want the heartbeat's timestamp")
	}

	var tableBuf bytes.Buffer
	if err := runStats(&tableBuf, statsOptions{DBPath: dbPath}); err != nil {
		t.Fatalf("runStats (table): %v", err)
	}
	if !strings.Contains(tableBuf.String(), "heartbeat") {
		t.Errorf("expected table output to mention heartbeat, got:\n%s", tableBuf.String())
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
	for _, flag := range []string{"--tool", "--agent", "--ticket", "--project", "--json"} {
		if !strings.Contains(out, flag) {
			t.Errorf("--help output missing %q; got:\n%s", flag, out)
		}
	}
}
