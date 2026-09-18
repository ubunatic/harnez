package telemetry

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "tool_catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

func sampleCall(session, tool string, score, exitCode int) ToolCall {
	return ToolCall{
		CreatedAt:      time.Now().UTC(),
		SessionID:      session,
		TicketID:       "116",
		ProjectName:    "harnez",
		WorkingDir:     "/home/uwe/projects/harnez",
		AgentID:        "claude",
		ToolName:       tool,
		CallType:       "internal",
		Score:          intPtr(score),
		Note:           "test call",
		ExitCode:       intPtr(exitCode),
		DurationMs:     5,
		RawBytes:       1000,
		DistilledBytes: int64Ptr(100),
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool_catalog.sqlite")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	// Second Open against the same, now-existing, file must not error —
	// schema creation is CREATE TABLE/INDEX IF NOT EXISTS.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db2.Close()
}

// TestOpenRejectsStaleSchemaVersion is the regression check for the gap
// found reviewing issue 120: CREATE TABLE IF NOT EXISTS silently leaves an
// existing file's older column shape untouched (this bit issue 118's
// distilled_bytes NOT NULL -> nullable change against a real pre-existing
// db). Open must fail with a clear, actionable message instead of letting
// a later Insert/Query hit a raw constraint or scan error.
func TestOpenMigratesStaleSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool_catalog.sqlite")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.sql.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatalf("reset user_version: %v", err)
	}
	db.Close()

	// Directly stamp a stale version, simulating a file created before a
	// schema-shape change: this must not go through Open again first,
	// since Open would just re-stamp a fresh 0 version to current.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := raw.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion-1)); err != nil {
		t.Fatalf("stamp stale version: %v", err)
	}
	raw.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatalf("Open should migrate stale schema: %v", err)
	}
	db.Close()
	if _, err := sql.Open("sqlite", path); err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
}

// TestOpenRejectsPreexistingTableWithUnstampedVersion is the regression
// check for a real production bug (2026-08-31): a file whose tool_calls
// table was created before this version-tracking guard existed has
// PRAGMA user_version == 0 — the exact same value a genuinely brand-new
// file has. The original guard treated any 0 as "fresh, safe to stamp,"
// so it silently accepted this repo's own real pre-existing
// ~/.harnez/tool_catalog.sqlite (still carrying the old NOT NULL
// distilled_bytes column) and stamped it as current, deferring the
// failure to a much less clear raw constraint error on the next Insert.
// Open must instead distinguish "this call's own CREATE TABLE just made
// the table" from "the table already existed" (see tableExists) and
// refuse to trust an unstamped-but-preexisting table.
func TestOpenMigratesPreexistingTableWithUnstampedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool_catalog.sqlite")

	// Simulate a file created by a pre-guard version of this package:
	// the old-shape table exists, but PRAGMA user_version was never
	// touched (still its SQLite default, 0) — do NOT go through this
	// package's own Open, which would correctly stamp a truly-fresh file.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// Mirrors issue 116's original (pre-118) DDL: every column schemaDDL's
	// own CREATE INDEX statements expect, just with the old NOT NULL
	// distilled_bytes this whole guard exists to catch.
	if _, err := raw.Exec(`CREATE TABLE tool_calls (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at      TEXT    NOT NULL,
		session_id      TEXT    NOT NULL,
		ticket_id       TEXT    NOT NULL DEFAULT '',
		project_name    TEXT    NOT NULL DEFAULT '',
		working_dir     TEXT    NOT NULL DEFAULT '',
		agent_id        TEXT    NOT NULL,
		tool_name       TEXT    NOT NULL,
		call_type       TEXT    NOT NULL,
		score           INTEGER,
		note            TEXT    NOT NULL DEFAULT '',
		exit_code       INTEGER,
		duration_ms     INTEGER NOT NULL DEFAULT 0,
		raw_bytes       INTEGER NOT NULL DEFAULT 0,
		distilled_bytes INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create legacy-shape table: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open should migrate preexisting table: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.sql.QueryRow("SELECT count(*) FROM pragma_table_info('tool_calls') WHERE name IN ('output_bytes', 'actual_tokens', 'potential_savings_tokens', 'potential_savings_bytes')").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("migrated column count = %d, want 4", count)
	}
}

func TestInsertQueryTelemetryRoundTrip(t *testing.T) {
	db := openTestDB(t)
	values := ToolCall{SessionID: "roundtrip", AgentID: "agent", ToolName: "test", CallType: "internal", OutputBytes: int64Ptr(11), ActualTokens: int64Ptr(22), PotentialSavingsTokens: int64Ptr(33), PotentialSavingsBytes: int64Ptr(44)}
	if err := db.Insert(values); err != nil {
		t.Fatal(err)
	}
	if err := db.Insert(ToolCall{SessionID: "nil", AgentID: "agent", ToolName: "test", CallType: "internal"}); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(Filter{ToolName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	var rich, empty ToolCall
	for _, row := range rows {
		if row.SessionID == "roundtrip" {
			rich = row
		} else {
			empty = row
		}
	}
	if *rich.OutputBytes != 11 || *rich.ActualTokens != 22 || *rich.PotentialSavingsTokens != 33 || *rich.PotentialSavingsBytes != 44 {
		t.Errorf("rich fields did not round-trip: %+v", rich)
	}
	if empty.OutputBytes != nil || empty.ActualTokens != nil || empty.PotentialSavingsTokens != nil || empty.PotentialSavingsBytes != nil {
		t.Errorf("nil fields became non-nil: %+v", empty)
	}
}

func TestInsertAndQuery(t *testing.T) {
	db := openTestDB(t)

	calls := []ToolCall{
		sampleCall("sess-1", "Read", 5, 0),
		sampleCall("sess-1", "Bash", 3, 1),
		sampleCall("sess-2", "Read", 4, 0),
	}
	for i, c := range calls {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert(%d): %v", i, err)
		}
	}

	got, err := db.Query(Filter{ToolName: "Read"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Query ToolName=Read: got %d rows, want 2", len(got))
	}
	for _, row := range got {
		if row.ToolName != "Read" {
			t.Errorf("unexpected tool_name %q in filtered result", row.ToolName)
		}
		if row.ID == 0 {
			t.Errorf("expected autoincrement id to be assigned, got 0")
		}
	}

	got, err = db.Query(Filter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("Query SessionID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Query SessionID=sess-1: got %d rows, want 2", len(got))
	}

	got, err = db.Query(Filter{})
	if err != nil {
		t.Fatalf("Query unfiltered: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Query unfiltered: got %d rows, want 3", len(got))
	}
}

func TestAggregate(t *testing.T) {
	db := openTestDB(t)

	for _, c := range []ToolCall{
		sampleCall("sess-1", "Read", 5, 0),
		sampleCall("sess-1", "Read", 3, 1),
		sampleCall("sess-1", "Read", 4, 0),
	} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	stats, err := db.Aggregate(Filter{ToolName: "Read"})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if stats.Count != 3 {
		t.Errorf("Count = %d, want 3", stats.Count)
	}
	wantAvg := (5.0 + 3.0 + 4.0) / 3.0
	if diff := stats.AvgScore - wantAvg; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("AvgScore = %v, want %v", stats.AvgScore, wantAvg)
	}
	if stats.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", stats.FailureCount)
	}
	if stats.TotalRawBytes != 3000 {
		t.Errorf("TotalRawBytes = %d, want 3000", stats.TotalRawBytes)
	}
	if stats.TotalDistilled != 300 {
		t.Errorf("TotalDistilled = %d, want 300", stats.TotalDistilled)
	}
}

func TestAggregateByTool(t *testing.T) {
	db := openTestDB(t)

	calls := []ToolCall{
		func() ToolCall {
			c := sampleCall("sess-1", "Read", 5, 0)
			c.ActualTokens = int64Ptr(100)
			c.PotentialSavingsTokens = int64Ptr(10)
			c.PotentialSavingsBytes = int64Ptr(100)
			return c
		}(),
		func() ToolCall {
			c := sampleCall("sess-1", "Read", 3, 1)
			c.ActualTokens = int64Ptr(200)
			c.PotentialSavingsTokens = int64Ptr(20)
			c.PotentialSavingsBytes = int64Ptr(200)
			return c
		}(),
		sampleCall("sess-1", "Edit", 4, 0),
	}
	for _, c := range calls {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	groups, err := db.AggregateByTool(Filter{})
	if err != nil {
		t.Fatalf("AggregateByTool: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	// ORDER BY COUNT(*) DESC, key ASC: Read (2 calls) before Edit (1 call).
	if groups[0].Key != "Read" || groups[0].Count != 2 {
		t.Errorf("groups[0] = %+v, want Key=Read Count=2", groups[0])
	}
	wantAvg := (5.0 + 3.0) / 2.0
	if diff := groups[0].AvgScore - wantAvg; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("groups[0].AvgScore = %v, want %v", groups[0].AvgScore, wantAvg)
	}
	if groups[0].FailureCount != 1 {
		t.Errorf("groups[0].FailureCount = %d, want 1", groups[0].FailureCount)
	}
	if groups[0].AvgActualTokens != 150 || groups[0].TotalPotentialSavingsTokens != 30 || groups[0].TotalPotentialSavingsBytes != 300 {
		t.Errorf("groups[0] analytical metrics = %+v, want avg tokens 150 and savings 30/300", groups[0])
	}
	if groups[1].Key != "Edit" || groups[1].Count != 1 {
		t.Errorf("groups[1] = %+v, want Key=Edit Count=1", groups[1])
	}
}

// TestAggregateByToolExcludesExpectedFailures covers issue 226: a shell
// row marked call_type=ExpectedFailureCallType still has a non-zero
// exit_code (a real, intentional failure) but must not count toward
// GroupStats.FailureCount, unlike an otherwise-identical unmarked failure.
func TestAggregateByToolExcludesExpectedFailures(t *testing.T) {
	db := openTestDB(t)

	expected := sampleCall("sess-1", "Bash", 5, 1)
	expected.CallType = ExpectedFailureCallType
	unmarked := sampleCall("sess-1", "Bash", 5, 1)
	unmarked.CallType = "shell"
	for _, c := range []ToolCall{expected, unmarked} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	groups, err := db.AggregateByTool(Filter{})
	if err != nil {
		t.Fatalf("AggregateByTool: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if groups[0].Count != 2 {
		t.Errorf("Count = %d, want 2 (both rows still counted)", groups[0].Count)
	}
	if groups[0].FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1 (only the unmarked failure)", groups[0].FailureCount)
	}
}

func TestAggregateByAgent(t *testing.T) {
	db := openTestDB(t)

	c1 := sampleCall("sess-1", "Read", 5, 0)
	c1.AgentID = "claude"
	c2 := sampleCall("sess-2", "Read", 2, 1)
	c2.AgentID = "codex"
	for _, c := range []ToolCall{c1, c2} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	groups, err := db.AggregateByAgent(Filter{})
	if err != nil {
		t.Fatalf("AggregateByAgent: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	byKey := map[string]GroupStats{}
	for _, g := range groups {
		byKey[g.Key] = g
	}
	if byKey["claude"].Count != 1 || byKey["codex"].Count != 1 {
		t.Errorf("byKey = %+v, want 1 call each", byKey)
	}
}

// TestAggregateByProject covers issue 227: a per-project breakdown,
// grouped on project_name (the stable identity across relocations of a
// checkout, per the ticket's Notes), mirroring TestAggregateByAgent.
func TestAggregateByProject(t *testing.T) {
	db := openTestDB(t)

	c1 := sampleCall("sess-1", "Read", 5, 0)
	c1.ProjectName = "harnez"
	c2 := sampleCall("sess-1", "Read", 3, 1)
	c2.ProjectName = "harnez"
	c3 := sampleCall("sess-2", "Edit", 4, 0)
	c3.ProjectName = "voxi"
	for _, c := range []ToolCall{c1, c2, c3} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	groups, err := db.AggregateByProject(Filter{})
	if err != nil {
		t.Fatalf("AggregateByProject: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	// ORDER BY COUNT(*) DESC, key ASC: harnez (2 calls) before voxi (1 call).
	if groups[0].Key != "harnez" || groups[0].Count != 2 {
		t.Errorf("groups[0] = %+v, want Key=harnez Count=2", groups[0])
	}
	wantAvg := (5.0 + 3.0) / 2.0
	if diff := groups[0].AvgScore - wantAvg; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("groups[0].AvgScore = %v, want %v", groups[0].AvgScore, wantAvg)
	}
	if groups[0].FailureCount != 1 {
		t.Errorf("groups[0].FailureCount = %d, want 1", groups[0].FailureCount)
	}
	if groups[1].Key != "voxi" || groups[1].Count != 1 {
		t.Errorf("groups[1] = %+v, want Key=voxi Count=1", groups[1])
	}
}

// TestFilterProject covers the Filter.Project field (issue 227): filters
// tool_calls rows down to one project_name, combining with other filters
// via AND like the pre-existing ToolName/AgentID/TicketID fields.
func TestFilterProject(t *testing.T) {
	db := openTestDB(t)

	c1 := sampleCall("sess-1", "Read", 5, 0)
	c1.ProjectName = "harnez"
	c2 := sampleCall("sess-2", "Read", 3, 1)
	c2.ProjectName = "voxi"
	for _, c := range []ToolCall{c1, c2} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	groups, err := db.AggregateByTool(Filter{Project: "harnez"})
	if err != nil {
		t.Fatalf("AggregateByTool: %v", err)
	}
	if len(groups) != 1 || groups[0].Count != 1 {
		t.Fatalf("groups = %+v, want 1 group with Count=1", groups)
	}
}

func TestAggregateByToolExcludesNullDistilledBytes(t *testing.T) {
	db := openTestDB(t)

	withDistill := sampleCall("sess-1", "Read", 5, 0)
	withDistill.RawBytes = 1000
	withDistill.DistilledBytes = int64Ptr(200)
	noDistill := sampleCall("sess-1", "Read", 4, 0)
	noDistill.RawBytes = 500
	noDistill.DistilledBytes = nil
	for _, c := range []ToolCall{withDistill, noDistill} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	groups, err := db.AggregateByTool(Filter{})
	if err != nil {
		t.Fatalf("AggregateByTool: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	// TotalDistilled sums only non-NULL distilled_bytes rows (SQL SUM
	// ignores NULLs); TotalRawBytes sums all rows regardless of whether
	// distillation ran. The global byte-savings ratio issue 120 needs is
	// computed by DistillationSavings instead (see TestDistillationSavings),
	// which restricts both sums to distilled_bytes IS NOT NULL rows so
	// they're directly comparable.
	if groups[0].TotalDistilled != 200 {
		t.Errorf("TotalDistilled = %d, want 200", groups[0].TotalDistilled)
	}
}

func TestDistillationSavings(t *testing.T) {
	db := openTestDB(t)

	withDistill := sampleCall("sess-1", "Bash", 5, 0)
	withDistill.RawBytes = 1000
	withDistill.DistilledBytes = int64Ptr(200) // 80% saved
	noDistill := sampleCall("sess-1", "Bash", 4, 0)
	noDistill.RawBytes = 999999 // must be excluded entirely, not counted as 0 saved
	noDistill.DistilledBytes = nil
	for _, c := range []ToolCall{withDistill, noDistill} {
		if err := db.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	ds, err := db.DistillationSavings(Filter{})
	if err != nil {
		t.Fatalf("DistillationSavings: %v", err)
	}
	if ds.Count != 1 {
		t.Fatalf("Count = %d, want 1 (only the distilled row)", ds.Count)
	}
	if ds.RawBytes != 1000 {
		t.Errorf("RawBytes = %d, want 1000", ds.RawBytes)
	}
	if ds.DistilledBytes != 200 {
		t.Errorf("DistilledBytes = %d, want 200", ds.DistilledBytes)
	}
	wantRatio := 1 - (200.0 / 1000.0)
	if diff := ds.Ratio - wantRatio; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("Ratio = %v, want %v", ds.Ratio, wantRatio)
	}
}

func TestDistillationSavingsNoRows(t *testing.T) {
	db := openTestDB(t)
	ds, err := db.DistillationSavings(Filter{})
	if err != nil {
		t.Fatalf("DistillationSavings: %v", err)
	}
	if ds.Count != 0 || ds.Ratio != 0 {
		t.Errorf("ds = %+v, want zero value", ds)
	}
}

// TestScoreConstraint confirms Insert does not hardcode a parallel "1-5"
// check in Go — it's the DB's own CHECK constraint (schema.go) that
// rejects an out-of-range score, and Insert just surfaces that error.
func TestScoreConstraint(t *testing.T) {
	db := openTestDB(t)

	bad := sampleCall("sess-1", "Read", 5, 0)
	bad.Score = intPtr(9)
	if err := db.Insert(bad); err == nil {
		t.Fatal("Insert with score=9 should have failed the schema's CHECK constraint")
	}
}

// TestConcurrentWriters is the regression check for the exact failure mode
// that disqualified DuckDB in issue 115: a second concurrent writer's
// connection must queue via WAL + busy_timeout, not fail Open() outright.
// It also exercises the cold-start race this ticket's canary found (two
// processes racing to create the same brand-new db file), which is why
// the writer runs as a real separate OS process via a test-helper
// subprocess rather than a second *DB in the same process.
func TestConcurrentWriters(t *testing.T) {
	if os.Getenv("TELEMETRY_WRITER_HELPER") == "1" {
		runWriterHelper(t)
		return
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "tool_catalog.sqlite")

	const nProcs = 2
	const nInserts = 25
	results := make(chan error, nProcs)
	for i := 0; i < nProcs; i++ {
		tag := "writer" + string(rune('A'+i))
		go func(tag string) {
			cmd := exec.Command(os.Args[0], "-test.run=TestConcurrentWriters")
			cmd.Env = append(os.Environ(),
				"TELEMETRY_WRITER_HELPER=1",
				"TELEMETRY_WRITER_PATH="+path,
				"TELEMETRY_WRITER_N="+itoa(nInserts),
				"TELEMETRY_WRITER_TAG="+tag,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				err = &writerErr{tag: tag, err: err, out: string(out)}
			}
			results <- err
		}(tag)
	}

	for i := 0; i < nProcs; i++ {
		if err := <-results; err != nil {
			t.Errorf("concurrent writer failed: %v", err)
		}
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open after concurrent writers: %v", err)
	}
	defer db.Close()
	got, err := db.Query(Filter{})
	if err != nil {
		t.Fatalf("Query after concurrent writers: %v", err)
	}
	if len(got) != nProcs*nInserts {
		t.Errorf("got %d rows after concurrent writers, want %d", len(got), nProcs*nInserts)
	}
}

type writerErr struct {
	tag string
	err error
	out string
}

func (e *writerErr) Error() string {
	return e.tag + ": " + e.err.Error() + "\n" + e.out
}

// runWriterHelper is the subprocess entrypoint TestConcurrentWriters
// re-execs itself as (via TELEMETRY_WRITER_HELPER=1), so each "writer" is
// a genuinely separate OS process against the same db file.
func runWriterHelper(t *testing.T) {
	path := os.Getenv("TELEMETRY_WRITER_PATH")
	tag := os.Getenv("TELEMETRY_WRITER_TAG")
	n := atoi(os.Getenv("TELEMETRY_WRITER_N"))

	db, err := Open(path)
	if err != nil {
		t.Fatalf("%s: Open: %v", tag, err)
	}
	defer db.Close()

	for i := 0; i < n; i++ {
		if err := db.Insert(sampleCall(tag, "Bash", 4, 0)); err != nil {
			t.Fatalf("%s: Insert %d: %v", tag, i, err)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
