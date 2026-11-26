package usagestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// testPairTolerance stands in for the values a caller loads from spec/usage.yaml.
var testPairTolerance = PairTolerance{Reset: time.Minute, Snapshot: time.Second}

func openTurnPairStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "turns.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	return s
}

// writePairBoundary stores one turn boundary the way the turn capture does:
// each window is stamped with the capture time.
func writePairBoundary(t *testing.T, s *Store, boundary string, at time.Time, cacheAge time.Duration, windows []Window) {
	t.Helper()
	for i := range windows {
		windows[i].Provider = "claude"
		windows[i].Source = "claude-api"
		windows[i].Freshness = "fresh"
		windows[i].ObservedAt = at
	}
	_, err := s.WriteTurnQuotaBoundary(context.Background(), TurnQuotaBoundary{
		SessionID: "session-1", Turn: 1, Boundary: boundary, Provider: "claude", CapturedAt: at,
		Source: "claude-api", Freshness: "fresh", CacheAgeMS: cacheAge.Milliseconds(), HasCache: true, Windows: windows,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Live readings of one window differ by under a second in reset_at (ticket
// 1301 M4). They are the same window and must still pair.
func TestTurnQuotaPairToleratesResetJitter(t *testing.T) {
	s := openTurnPairStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 26, 10, 52, 44, 323000000, time.UTC)
	fiveBefore := time.Date(2026, 11, 26, 12, 10, 0, 65000000, time.UTC)
	fiveAfter := time.Date(2026, 11, 26, 12, 9, 59, 889000000, time.UTC)
	weekBefore := time.Date(2026, 11, 26, 11, 0, 0, 65000000, time.UTC)
	weekAfter := time.Date(2026, 11, 26, 10, 59, 59, 889000000, time.UTC)
	writePairBoundary(t, s, "before", at, 0, []Window{
		{Name: "5-hour", UsedFraction: .21, ResetAt: &fiveBefore},
		{Name: "Weekly (7-day)", UsedFraction: .10, ResetAt: &weekBefore},
	})
	writePairBoundary(t, s, "after", at.Add(5*time.Minute), 0, []Window{
		{Name: "5-hour", UsedFraction: .33, ResetAt: &fiveAfter},
		{Name: "Weekly (7-day)", UsedFraction: .12, ResetAt: &weekAfter},
	})
	if err := s.PairTurnQuotaDeltas(ctx, "session-1", 1, "claude", testPairTolerance); err != nil {
		t.Fatal(err)
	}
	deltas, err := s.TurnQuotaDeltas(ctx, "session-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 2 {
		t.Fatalf("paired deltas = %+v; want both windows paired despite reset jitter", deltas)
	}
	if outcome, _, ok, err := s.TurnQuotaPairOutcome(ctx, "session-1", 1, "claude"); err != nil || !ok || outcome != "paired" {
		t.Fatalf("outcome = %q, ok=%v, err=%v; want paired", outcome, ok, err)
	}
}

// A real rollover moves a window's reset by days. That window must not pair,
// while the jittered 5-hour window still does.
func TestTurnQuotaPairStillSplitsAtWeeklyRollover(t *testing.T) {
	s := openTurnPairStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 26, 10, 52, 44, 323000000, time.UTC)
	fiveBefore := time.Date(2026, 11, 26, 12, 10, 0, 65000000, time.UTC)
	fiveAfter := time.Date(2026, 11, 26, 12, 9, 59, 889000000, time.UTC)
	weekBefore := time.Date(2026, 11, 26, 11, 0, 0, 65000000, time.UTC)
	weekRolled := weekBefore.Add(7 * 24 * time.Hour)
	writePairBoundary(t, s, "before", at, 0, []Window{
		{Name: "5-hour", UsedFraction: .21, ResetAt: &fiveBefore},
		{Name: "Weekly (7-day)", UsedFraction: .10, ResetAt: &weekBefore},
	})
	writePairBoundary(t, s, "after", at.Add(5*time.Minute), 0, []Window{
		{Name: "5-hour", UsedFraction: .33, ResetAt: &fiveAfter},
		{Name: "Weekly (7-day)", UsedFraction: .02, ResetAt: &weekRolled},
	})
	if err := s.PairTurnQuotaDeltas(ctx, "session-1", 1, "claude", testPairTolerance); err != nil {
		t.Fatal(err)
	}
	deltas, err := s.TurnQuotaDeltas(ctx, "session-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 || deltas[0].WindowKey != "five_hour" {
		t.Fatalf("paired deltas = %+v; want only the five-hour window, not the rolled-over week", deltas)
	}
}

// The after reading reused the cached snapshot: both readings describe the same
// snapshot (captured_at minus cache_age_ms), with live timings. The turn records
// unchanged_snapshot and writes no 0-delta row.
func TestTurnQuotaPairSameSnapshotWritesNoDelta(t *testing.T) {
	s := openTurnPairStore(t)
	ctx := context.Background()
	snapshot := time.Date(2026, 11, 26, 10, 43, 38, 227000000, time.UTC)
	reset := snapshot.Add(5 * time.Hour)
	beforeAge := 546096 * time.Millisecond
	afterAge := 549050 * time.Millisecond
	writePairBoundary(t, s, "before", snapshot.Add(beforeAge), beforeAge, []Window{
		{Name: "5-hour", UsedFraction: .21, ResetAt: &reset},
	})
	writePairBoundary(t, s, "after", snapshot.Add(afterAge), afterAge, []Window{
		{Name: "5-hour", UsedFraction: .21, ResetAt: &reset},
	})
	if err := s.PairTurnQuotaDeltas(ctx, "session-1", 1, "claude", testPairTolerance); err != nil {
		t.Fatal(err)
	}
	deltas, err := s.TurnQuotaDeltas(ctx, "session-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 0 {
		t.Fatalf("deltas = %+v; want none for a reused snapshot", deltas)
	}
	if outcome, _, ok, err := s.TurnQuotaPairOutcome(ctx, "session-1", 1, "claude"); err != nil || !ok || outcome != "unchanged_snapshot" {
		t.Fatalf("outcome = %q, ok=%v, err=%v; want unchanged_snapshot", outcome, ok, err)
	}
}
