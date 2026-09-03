package telemetry

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestDefaultLocalClassifier_LiveManual is an opt-in diagnostic test that
// exercises DefaultLocalClassifier against the real tool_catalog.sqlite
// database and whatever local model server is actually running (e.g.
// `lmcoder start`). It never runs as part of `go test ./...` — set
// RUN_LIVE_CLASSIFY_TEST=1 to enable it manually.
//
// Unlike ClassifyNotes, which swallows Tier 3 batch errors and falls back to
// CategoryOther, this test surfaces the classifier's actual error and raw
// per-note results so failures in the real batch/parse path (as opposed to
// the httptest-mocked path in classify_test.go) are visible.
func TestDefaultLocalClassifier_LiveManual(t *testing.T) {
	if os.Getenv("RUN_LIVE_CLASSIFY_TEST") != "1" {
		t.Skip("set RUN_LIVE_CLASSIFY_TEST=1 to run against the real DB and a live local model server")
	}

	dbPath, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%s): %v", dbPath, err)
	}
	defer db.Close()

	rows, err := db.Query(Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	seen := make(map[string]bool)
	var misses []string
	for _, r := range rows {
		if r.Note == "" {
			continue
		}
		if cat := ClassifyTier1(r.ToolName, r.Note, r.ExitCode); cat == CategoryOther && !seen[r.Note] {
			seen[r.Note] = true
			misses = append(misses, r.Note)
		}
	}
	t.Logf("tier1 misses (distinct): %d", len(misses))
	if len(misses) == 0 {
		t.Skip("no Tier 1 misses in the current DB to classify")
	}

	// Batch size mirrors ClassifyNotes' production batchSize (see classify.go)
	// by default, but is tunable via CLASSIFY_LIVE_BATCH_SIZE to bisect how
	// batch size trades off against wall-clock time on the local model.
	batchSize := 50
	if v := os.Getenv("CLASSIFY_LIVE_BATCH_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("invalid CLASSIFY_LIVE_BATCH_SIZE=%q: %v", v, err)
		}
		batchSize = n
	}
	end := batchSize
	if end > len(misses) {
		end = len(misses)
	}
	batch := misses[:end]

	timeout := 90 * time.Second
	if v := os.Getenv("CLASSIFY_LIVE_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("invalid CLASSIFY_LIVE_TIMEOUT_SECONDS=%q: %v", v, err)
		}
		timeout = time.Duration(n) * time.Second
	}

	c := &DefaultLocalClassifier{Timeout: timeout}
	start := time.Now()
	cats, err := c.ClassifyBatch(context.Background(), batch)
	elapsed := time.Since(start)
	t.Logf("ClassifyBatch(%d notes) took %s (timeout %s)", len(batch), elapsed, timeout)
	if err != nil {
		t.Fatalf("ClassifyBatch failed on %d-note batch after %s: %v", len(batch), elapsed, err)
	}
	if len(cats) != len(batch) {
		t.Fatalf("ClassifyBatch returned %d categories for %d notes", len(cats), len(batch))
	}
	for i, cat := range cats {
		t.Logf("note=%q -> category=%s", batch[i], cat)
	}
}
