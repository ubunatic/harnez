package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// lastRow opens dbPath and returns the newest row.
func lastRow(t *testing.T, dbPath string) telemetry.ToolCall {
	t.Helper()
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no rows written")
	}
	return rows[0]
}

func TestRunRate_ParsesPositionalArgsAndWritesRow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	err := runRate([]string{"Read", "4", "read the spec file", "myproj/117-rate"}, rateOptions{
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
	})
	if err != nil {
		t.Fatalf("runRate() error = %v", err)
	}

	row := lastRow(t, dbPath)
	if row.ToolName != "Read" {
		t.Errorf("ToolName = %q, want %q", row.ToolName, "Read")
	}
	if row.Score == nil || *row.Score != 4 {
		t.Errorf("Score = %v, want 4", row.Score)
	}
	if row.Note != "read the spec file" {
		t.Errorf("Note = %q, want %q", row.Note, "read the spec file")
	}
	if row.TicketID != "myproj/117-rate" {
		t.Errorf("TicketID = %q, want %q", row.TicketID, "myproj/117-rate")
	}
	if row.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", row.SessionID, "sess-1")
	}
	if row.CallType != "internal" {
		t.Errorf("CallType = %q, want %q", row.CallType, "internal")
	}
	if row.ExitCode != nil {
		t.Errorf("ExitCode = %v, want nil", row.ExitCode)
	}
}

func TestRunRate_RejectsOutOfRangeScore(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	for _, score := range []string{"0", "6", "-1"} {
		err := runRate([]string{"Read", score, "desc", "proj/1-t"}, rateOptions{
			SessionFlag: "sess-1",
			DBPath:      dbPath,
			StateDir:    filepath.Join(tmp, "state"),
		})
		if err == nil {
			t.Fatalf("runRate(score=%s) error = nil, want out-of-range error", score)
		}
		if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "5") {
			t.Errorf("runRate(score=%s) error %q does not name the valid range 1-5", score, err.Error())
		}
	}
}

func TestRunRate_RejectsNonIntegerScore(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	err := runRate([]string{"Read", "great", "desc", "proj/1-t"}, rateOptions{
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
	})
	if err == nil {
		t.Fatal("runRate(score=\"great\") error = nil, want a parse error")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error %q does not mention that score must be an integer", err.Error())
	}
}

func TestRunRate_AgentFlagOverride(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	err := runRate([]string{"Read", "3", "desc", "proj/1-t"}, rateOptions{
		AgentFlag:   "my-custom-agent",
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		Getenv:      func(string) string { return "" }, // no env signals; flag must still win
	})
	if err != nil {
		t.Fatalf("runRate() error = %v", err)
	}
	if row := lastRow(t, dbPath); row.AgentID != "my-custom-agent" {
		t.Errorf("AgentID = %q, want %q", row.AgentID, "my-custom-agent")
	}
}

func TestRunRate_AgentAutoDetectFromEnv(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	getenv := func(k string) string {
		if k == "CLAUDE_CODE_SESSION_ID" {
			return "some-session"
		}
		return ""
	}
	err := runRate([]string{"Read", "3", "desc", "proj/1-t"}, rateOptions{
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		Getenv:      getenv,
	})
	if err != nil {
		t.Fatalf("runRate() error = %v", err)
	}
	if row := lastRow(t, dbPath); row.AgentID != "claude" {
		t.Errorf("AgentID = %q, want %q (auto-detected)", row.AgentID, "claude")
	}
}

func TestRunRate_AgentDefaultsUnknown(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	err := runRate([]string{"Read", "3", "desc", "proj/1-t"}, rateOptions{
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		Getenv:      func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("runRate() error = %v", err)
	}
	if row := lastRow(t, dbPath); row.AgentID != "unknown" {
		t.Errorf("AgentID = %q, want %q", row.AgentID, "unknown")
	}
}

func TestRunRate_SessionFlagOverride(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	err := runRate([]string{"Read", "3", "desc", "proj/1-t"}, rateOptions{
		SessionFlag: "explicit-session-42",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		Getenv:      func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("runRate() error = %v", err)
	}
	if row := lastRow(t, dbPath); row.SessionID != "explicit-session-42" {
		t.Errorf("SessionID = %q, want %q", row.SessionID, "explicit-session-42")
	}
}

// TestRunRate_OmittedTicketResolvesViaResolve is the acceptance-criteria
// test for "omitted ticket_id resolves via internal/resolve rather than
// silently writing NULL": a first call passes an explicit ticket_id, and
// a second call in the same session omits it — the second call's row must
// inherit the first's ticket_id (internal/resolve's only remaining
// implicit-resolution path since the 2026-08-31 simplification: this repo
// never branches per ticket, so branch-name inference was removed).
func TestRunRate_OmittedTicketResolvesViaResolve(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")
	opts := rateOptions{
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		Getenv:      func(string) string { return "" },
	}

	if err := runRate([]string{"Edit", "5", "desc", "myproject/117-harnez-rate-command"}, opts); err != nil {
		t.Fatalf("first runRate() error = %v", err)
	}
	if err := runRate([]string{"Read", "5", "desc"}, opts); err != nil { // no ticket_id positional
		t.Fatalf("second runRate() error = %v", err)
	}

	want := "myproject/117-harnez-rate-command"
	if row := lastRow(t, dbPath); row.TicketID != want {
		t.Errorf("TicketID = %q, want %q (inherited, not NULL/empty)", row.TicketID, want)
	}
}

// TestRunRate_OmittedTicketWithNoHistoryWritesEmpty is the regression check
// for the 2026-08-31 fix: an unresolvable ticket_id (no explicit arg, no
// prior session history — the normal case for this repo, which always
// works on its default branch) must NOT error and must NOT drop the row.
// It previously did both, via internal/resolve.Ticket's now-removed
// branch-name-heuristic-or-error behavior.
func TestRunRate_OmittedTicketWithNoHistoryWritesEmpty(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")

	err := runRate([]string{"Read", "5", "desc"}, rateOptions{
		SessionFlag: "sess-never-used-before",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		Getenv:      func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("runRate() error = %v, want nil (unresolved ticket_id is not an error)", err)
	}
	if row := lastRow(t, dbPath); row.TicketID != "" {
		t.Errorf("TicketID = %q, want empty (no history to inherit)", row.TicketID)
	}
}

// TestRunRate_LatencyBudget is the end-to-end (arg parse -> resolve ->
// open -> insert) latency benchmark for issue 117's sub-20ms acceptance
// criterion. It covers both a cold DB (first call, schema creation) and
// repeated warm calls against the same DB file.
func TestRunRate_LatencyBudget(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")
	stateDir := filepath.Join(tmp, "state")
	getenv := func(string) string { return "" }

	const budgetMs = 20

	coldStart := time.Now()
	if err := runRate([]string{"Read", "4", "cold call", "proj/1-t"}, rateOptions{
		SessionFlag: "sess-1", DBPath: dbPath, StateDir: stateDir, Getenv: getenv,
	}); err != nil {
		t.Fatalf("cold runRate() error = %v", err)
	}
	coldMs := float64(time.Since(coldStart)) / float64(time.Millisecond)
	if coldMs > budgetMs {
		t.Errorf("cold end-to-end runRate took %.2fms, want sub-%dms", coldMs, budgetMs)
	}
	t.Logf("cold end-to-end runRate: %.2fms", coldMs)

	const warmN = 50
	var maxWarmMs float64
	for i := 0; i < warmN; i++ {
		start := time.Now()
		if err := runRate([]string{"Read", "4", "warm call", "proj/1-t"}, rateOptions{
			SessionFlag: "sess-1", DBPath: dbPath, StateDir: stateDir, Getenv: getenv,
		}); err != nil {
			t.Fatalf("warm runRate() %d error = %v", i, err)
		}
		ms := float64(time.Since(start)) / float64(time.Millisecond)
		if ms > maxWarmMs {
			maxWarmMs = ms
		}
		if ms > budgetMs {
			t.Errorf("warm runRate() %d took %.2fms, want sub-%dms", i, ms, budgetMs)
		}
	}
	t.Logf("warm end-to-end runRate: max %.2fms over %d calls", maxWarmMs, warmN)
}

// BenchmarkRunRate is the durable regression benchmark counterpart to
// TestRunRate_LatencyBudget, mirroring internal/telemetry's
// BenchmarkInsert. Run with:
//
//	go test ./cmd/harnez/... -bench=BenchmarkRunRate -benchtime=200x
func BenchmarkRunRate(b *testing.B) {
	tmp := b.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")
	stateDir := filepath.Join(tmp, "state")
	getenv := func(string) string { return "" }

	// Warm the DB (schema creation) before timing.
	if err := runRate([]string{"Read", "4", "warmup", "proj/1-t"}, rateOptions{
		SessionFlag: "sess-1", DBPath: dbPath, StateDir: stateDir, Getenv: getenv,
	}); err != nil {
		b.Fatalf("warmup runRate() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := runRate([]string{"Read", "4", "bench call", "proj/1-t"}, rateOptions{
			SessionFlag: "sess-1", DBPath: dbPath, StateDir: stateDir, Getenv: getenv,
		}); err != nil {
			b.Fatalf("runRate() error = %v", err)
		}
	}
}
