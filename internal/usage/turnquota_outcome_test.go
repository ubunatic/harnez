package usage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

// quotaOutcomeFixture stores a before reading at t0 and returns the store path,
// the reset time, and a weekly window observed at t0.
func quotaOutcomeFixture(t *testing.T) (dbPath string, t0 time.Time, reset time.Time, before usagestore.TurnQuotaBoundary) {
	t.Helper()
	dbPath = filepath.Join(t.TempDir(), "telemetry.sqlite")
	t0 = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	reset = t0.Add(48 * time.Hour)
	win := usagestore.Window{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.10, ResetAt: &reset, ObservedAt: t0, Source: "registry"}
	before = usagestore.TurnQuotaBoundary{SessionID: "s-out", Turn: 1, Boundary: "before", Provider: "claude", CapturedAt: t0, Source: "registry", Freshness: "live", HasCache: true, Windows: []usagestore.Window{win}}
	if err := PersistTurnQuotaBoundary(context.Background(), dbPath, before); err != nil {
		t.Fatal(err)
	}
	return dbPath, t0, reset, before
}

func quotaOutcomeFor(t *testing.T, dbPath string) (string, string) {
	t.Helper()
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	outcome, detail, ok, err := store.TurnQuotaPairOutcome(context.Background(), "s-out", 1, "claude")
	if err != nil || !ok {
		t.Fatalf("TurnQuotaPairOutcome ok=%v err=%v; want a recorded outcome", ok, err)
	}
	return outcome, detail
}

func deltaCountFor(t *testing.T, dbPath string) int {
	t.Helper()
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	deltas, err := store.TurnQuotaDeltasForSession(context.Background(), "s-out")
	if err != nil {
		t.Fatal(err)
	}
	return len(deltas)
}

// The after reading reuses the cached snapshot: same observation, so no new
// window row. The outcome says so instead of silently writing nothing.
func TestTurnQuotaPairOutcomeUnchangedSnapshot(t *testing.T) {
	dbPath, t0, reset, before := quotaOutcomeFixture(t)
	after := before
	after.Boundary = "after"
	after.CapturedAt = t0.Add(30 * time.Second)
	after.CacheAgeMS = 30000
	after.Windows = []usagestore.Window{{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.10, ResetAt: &reset, ObservedAt: t0, Source: "registry"}}
	if err := PersistTurnQuotaBoundary(context.Background(), dbPath, after); err != nil {
		t.Fatal(err)
	}
	if outcome, _ := quotaOutcomeFor(t, dbPath); outcome != "unchanged_snapshot" {
		t.Fatalf("outcome = %q; want unchanged_snapshot for a reused cached reading", outcome)
	}
	if n := deltaCountFor(t, dbPath); n != 0 {
		t.Fatalf("delta rows = %d; want none, not a 0-delta row", n)
	}
}

// A failed fetch (HTTP 429) is recorded with its error; it is not a 0 delta.
func TestTurnQuotaPairOutcomeFetchErrorIsRecorded(t *testing.T) {
	dbPath, t0, _, before := quotaOutcomeFixture(t)
	after := before
	after.Boundary = "after"
	after.CapturedAt = t0.Add(30 * time.Second)
	after.HasCache = false
	after.Windows = nil
	after.Error = "usage API HTTP 429"
	if err := PersistTurnQuotaBoundary(context.Background(), dbPath, after); err != nil {
		t.Fatal(err)
	}
	outcome, detail := quotaOutcomeFor(t, dbPath)
	if outcome != "fetch_error" || !strings.Contains(detail, "429") {
		t.Fatalf("outcome = %q, detail = %q; want fetch_error carrying the 429", outcome, detail)
	}
	if n := deltaCountFor(t, dbPath); n != 0 {
		t.Fatalf("delta rows = %d; want none after a 429", n)
	}
}

// A fresh after reading pairs with the before reading and writes one delta.
func TestTurnQuotaPairOutcomePairedWritesDelta(t *testing.T) {
	dbPath, t0, reset, before := quotaOutcomeFixture(t)
	after := before
	after.Boundary = "after"
	after.CapturedAt = t0.Add(30 * time.Second)
	after.Windows = []usagestore.Window{{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.12, ResetAt: &reset, ObservedAt: t0.Add(30 * time.Second), Source: "registry"}}
	if err := PersistTurnQuotaBoundary(context.Background(), dbPath, after); err != nil {
		t.Fatal(err)
	}
	if outcome, _ := quotaOutcomeFor(t, dbPath); outcome != "paired" {
		t.Fatalf("outcome = %q; want paired", outcome)
	}
	if n := deltaCountFor(t, dbPath); n != 1 {
		t.Fatalf("delta rows = %d; want 1", n)
	}
}
