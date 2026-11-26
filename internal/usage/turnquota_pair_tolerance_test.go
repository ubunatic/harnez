package usage

import (
	"context"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

// The live write path pairs a reset that differs by under a second (ticket 1301
// M4). The tolerance comes from spec/usage.yaml, so this exercises the spec load.
func TestTurnQuotaPairJitteredResetPairsThroughSpecTolerance(t *testing.T) {
	dbPath, t0, reset, _ := quotaOutcomeFixture(t, 0)
	afterAt := t0.Add(30 * time.Second)
	jittered := reset.Add(-176 * time.Millisecond)
	after := usagestore.TurnQuotaBoundary{SessionID: "s-out", Turn: 1, Boundary: "after", Provider: "claude", CapturedAt: afterAt, Source: "registry", Freshness: "live", HasCache: true}
	after.Windows = []usagestore.Window{{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.12, ResetAt: &jittered, ObservedAt: afterAt, Source: "registry"}}
	if err := PersistTurnQuotaBoundary(context.Background(), dbPath, after); err != nil {
		t.Fatal(err)
	}
	if outcome, _ := quotaOutcomeFor(t, dbPath); outcome != "paired" {
		t.Fatalf("outcome = %q; want paired across 176 ms of reset jitter", outcome)
	}
	if n := deltaCountFor(t, dbPath); n != 1 {
		t.Fatalf("delta rows = %d; want 1", n)
	}
}

// A window that rolled over by a week is a new window, not jitter: it must not
// pair, and the outcome says why.
func TestTurnQuotaPairRolledOverWindowDoesNotPairThroughSpec(t *testing.T) {
	dbPath, t0, reset, _ := quotaOutcomeFixture(t, 0)
	afterAt := t0.Add(30 * time.Second)
	rolled := reset.Add(7 * 24 * time.Hour)
	after := usagestore.TurnQuotaBoundary{SessionID: "s-out", Turn: 1, Boundary: "after", Provider: "claude", CapturedAt: afterAt, Source: "registry", Freshness: "live", HasCache: true}
	after.Windows = []usagestore.Window{{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.02, ResetAt: &rolled, ObservedAt: afterAt, Source: "registry"}}
	if err := PersistTurnQuotaBoundary(context.Background(), dbPath, after); err != nil {
		t.Fatal(err)
	}
	if outcome, _ := quotaOutcomeFor(t, dbPath); outcome != "no_pair" {
		t.Fatalf("outcome = %q; want no_pair for a rolled-over window", outcome)
	}
	if n := deltaCountFor(t, dbPath); n != 0 {
		t.Fatalf("delta rows = %d; want none across a rollover", n)
	}
}
