package telemetry

import (
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkInsert is the durable regression check for the sub-20ms
// single-row insert budget this ticket's canary measured manually
// (avg 0.91ms / max 6.1ms on real disk). Run with:
//
//	go test ./internal/telemetry/... -bench=BenchmarkInsert -benchtime=200x
func BenchmarkInsert(b *testing.B) {
	dir := b.TempDir()
	db, err := Open(filepath.Join(dir, "tool_catalog.sqlite"))
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	defer db.Close()

	call := sampleCall("bench-session", "Bash", 4, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.Insert(call); err != nil {
			b.Fatalf("Insert: %v", err)
		}
	}
}

// TestInsertLatencyBudget is a non-benchmark assertion of the sub-20ms
// acceptance criterion, so `go test` (not just `go test -bench`) fails
// loudly if insert latency regresses.
func TestInsertLatencyBudget(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "tool_catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	const n = 100
	const budgetMs = 20
	for i := 0; i < n; i++ {
		call := sampleCall("latency-session", "Bash", 4, 0)
		start := time.Now()
		if err := db.Insert(call); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
		elapsedMs := float64(time.Since(start)) / float64(time.Millisecond)
		if elapsedMs > budgetMs {
			t.Errorf("insert %d took %.2fms, want sub-%dms", i, elapsedMs, budgetMs)
		}
	}
}
