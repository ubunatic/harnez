package telemetry

import (
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

func intPtr(v int) *int { return &v }

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
		DistilledBytes: 100,
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
