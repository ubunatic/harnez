package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

// compactQuotaCollector isolates one collector DB and replaces the quota read,
// so no test reaches the live usage endpoint (issue 1303 M2). Samples run
// inline instead of in a goroutine.
type compactQuotaCollector struct {
	dbPath string
	reads  int
}

func installCompactQuotaCollector(t *testing.T, read func(context.Context, time.Time) usage.TurnQuotaReading) *compactQuotaCollector {
	t.Helper()
	root := t.TempDir()
	for _, key := range []string{"HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_CONFIG_HOME"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	dbPath := filepath.Join(root, "telemetry.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	c := &compactQuotaCollector{dbPath: dbPath}
	prevCapture, prevAsync := compactQuotaCapture, compactQuotaAsync
	compactQuotaCapture = func(ctx context.Context, now time.Time) usage.TurnQuotaReading {
		c.reads++
		return read(ctx, now)
	}
	compactQuotaAsync = func(fn func()) { fn() }
	t.Cleanup(func() { compactQuotaCapture, compactQuotaAsync = prevCapture, prevAsync })
	return c
}

// postCompactRecord is a collector record for a PostCompact hook payload. claude
// sets harnez_context.claude, which is how the collector tells Claude's row apart.
func postCompactRecord(recordID, sessionID string, claude bool) server.TelemetryRecord {
	payload, _ := json.Marshal(map[string]any{
		"session_id":      sessionID,
		"hook_event_name": "PostCompact",
		"trigger":         "manual",
		"harnez_context":  map[string]any{"claude": claude, "created_at": time.Now().UTC().Format(time.RFC3339Nano)},
	})
	return server.TelemetryRecord{RecordID: recordID, Kind: "codex", Payload: payload}
}

func (c *compactQuotaCollector) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	raw, err := sql.Open("sqlite", c.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func okQuotaRead(ctx context.Context, now time.Time) usage.TurnQuotaReading {
	return usage.TurnQuotaReading{CapturedAt: now, Source: "compact-fetch", HasCache: true, Windows: []usage.QuotaHistoryEntry{{Window: "5h", UsedPercent: 39}}}
}

// TestCompactQuotaTwoCompactionsWriteTwoSamples: each Claude PostCompact stores
// one compact sample, and a replayed record stores none again.
func TestCompactQuotaTwoCompactionsWriteTwoSamples(t *testing.T) {
	c := installCompactQuotaCollector(t, okQuotaRead)
	for _, rec := range []server.TelemetryRecord{postCompactRecord("rec-1", "sess-a", true), postCompactRecord("rec-2", "sess-a", true)} {
		if _, err := processTelemetryRecordResult(context.Background(), rec, c.dbPath); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := processTelemetryRecordResult(context.Background(), postCompactRecord("rec-1", "sess-a", true), c.dbPath); err != nil {
		t.Fatal(err)
	}
	if n := c.count(t, `SELECT COUNT(*) FROM turn_quota_boundaries WHERE boundary='compact' AND turn=0 AND session_id='sess-a'`); n != 2 {
		t.Fatalf("compact samples = %d, want 2 (one per compaction, replay adds none)", n)
	}
	if c.reads != 3 {
		t.Fatalf("quota reads = %d, want 3 (one per delivery; only the stored rows are deduplicated)", c.reads)
	}
}

// TestCompactQuotaTimeoutLeavesCompactionRowsUnchanged: a failed sample is stored
// as its error and writes no compaction row of its own.
func TestCompactQuotaTimeoutLeavesCompactionRowsUnchanged(t *testing.T) {
	timedOut := func(ctx context.Context, now time.Time) usage.TurnQuotaReading {
		return usage.TurnQuotaReading{CapturedAt: now, Source: "compact-fetch", Error: fmt.Sprintf("quota fetch: %v", context.DeadlineExceeded)}
	}
	c := installCompactQuotaCollector(t, timedOut)
	if _, err := processTelemetryRecordResult(context.Background(), postCompactRecord("rec-t", "sess-t", true), c.dbPath); err != nil {
		t.Fatal(err)
	}
	if n := c.count(t, `SELECT COUNT(*) FROM turn_quota_boundaries WHERE boundary='compact' AND error<>''`); n != 1 {
		t.Fatalf("errored compact samples = %d, want 1", n)
	}
	if n := c.count(t, `SELECT COUNT(*) FROM compaction_events WHERE session_id='sess-t'`); n != 1 {
		t.Fatalf("compaction rows = %d, want 1 (the sample must not change them)", n)
	}
	if n := c.count(t, `SELECT COUNT(*) FROM session_boundaries WHERE session_id='sess-t'`); n != 1 {
		t.Fatalf("session boundary rows = %d, want 1", n)
	}
}

// TestCompactQuotaSkipsNonClaudeRecords: a Codex PostCompact and a Claude record
// that is not a PostCompact take no sample.
func TestCompactQuotaSkipsNonClaudeRecords(t *testing.T) {
	c := installCompactQuotaCollector(t, okQuotaRead)
	for _, rec := range []server.TelemetryRecord{postCompactRecord("rec-c", "sess-c", false), postCompactRecord("", "sess-c", true)} {
		if _, err := processTelemetryRecordResult(context.Background(), rec, c.dbPath); err != nil {
			t.Fatal(err)
		}
	}
	if c.reads != 0 {
		t.Fatalf("quota reads = %d for non-Claude records, want 0", c.reads)
	}
}
