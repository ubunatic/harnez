package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// initRateTestRepo creates a bare-bones git repo (.git/HEAD only, enough
// for internal/resolve.Ticket's findRepoRoot/readBranch) checked out to
// branch, mirroring internal/resolve/resolve_test.go's initRepo helper.
func initRateTestRepo(t *testing.T, parent, name, branch string) string {
	t.Helper()
	root := filepath.Join(parent, name)
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	head := "ref: refs/heads/" + branch + "\n"
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(head), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

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
// silently writing NULL": no positional ticket_id is passed, but TicketDir
// points at a repo checked out to a ticket-shaped branch, so the resolved
// ticket_id must show up in the written row (never "" / NULL).
func TestRunRate_OmittedTicketResolvesViaResolve(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")
	repoRoot := initRateTestRepo(t, tmp, "myproject", "117-harnez-rate-command")

	err := runRate([]string{"Read", "5", "desc"}, rateOptions{ // no ticket_id positional
		SessionFlag: "sess-1",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		TicketDir:   repoRoot,
		Getenv:      func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("runRate() error = %v", err)
	}

	want := "myproject/117-harnez-rate-command"
	if row := lastRow(t, dbPath); row.TicketID != want {
		t.Errorf("TicketID = %q, want %q (resolved, not NULL/empty)", row.TicketID, want)
	}
}

func TestRunRate_OmittedTicketErrorsWhenUnresolvable(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "tool_catalog.sqlite")
	// TicketDir is a plain, non-git directory: resolve.Ticket has nothing
	// to resolve from and no prior session ticket history.
	plainDir := filepath.Join(tmp, "not-a-repo")
	if err := os.MkdirAll(plainDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := runRate([]string{"Read", "5", "desc"}, rateOptions{
		SessionFlag: "sess-never-used-before",
		DBPath:      dbPath,
		StateDir:    filepath.Join(tmp, "state"),
		TicketDir:   plainDir,
		Getenv:      func(string) string { return "" },
	})
	if err == nil {
		t.Fatal("runRate() error = nil, want an error since ticket_id cannot be resolved")
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
