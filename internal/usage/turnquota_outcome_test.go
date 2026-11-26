package usage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

// quotaOutcomeFixture stores a before reading whose cached snapshot was taken at
// t0 and whose capture happened cacheAge later, as the turn capture writes it.
// It returns the store path, the reset time, and the before boundary.
func quotaOutcomeFixture(t *testing.T, cacheAge time.Duration) (dbPath string, t0 time.Time, reset time.Time, before usagestore.TurnQuotaBoundary) {
	t.Helper()
	dbPath = filepath.Join(t.TempDir(), "telemetry.sqlite")
	t0 = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	reset = t0.Add(48 * time.Hour)
	captured := t0.Add(cacheAge)
	win := usagestore.Window{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.10, ResetAt: &reset, ObservedAt: captured, Source: "registry"}
	before = usagestore.TurnQuotaBoundary{SessionID: "s-out", Turn: 1, Boundary: "before", Provider: "claude", CapturedAt: captured, CacheAgeMS: cacheAge.Milliseconds(), Source: "registry", Freshness: "live", HasCache: true, Windows: []usagestore.Window{win}}
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

// The after reading reuses the cached snapshot taken at t0. Its capture time
// and cache age are the live ones (ticket 1301 M4), so both readings give the
// same snapshot time and the outcome says so instead of writing a 0 delta.
func TestTurnQuotaPairOutcomeUnchangedSnapshot(t *testing.T) {
	dbPath, t0, reset, _ := quotaOutcomeFixture(t, 546096*time.Millisecond)
	afterAt := t0.Add(549050 * time.Millisecond)
	after := usagestore.TurnQuotaBoundary{SessionID: "s-out", Turn: 1, Boundary: "after", Provider: "claude", CapturedAt: afterAt, CacheAgeMS: 549050, Source: "registry", Freshness: "live", HasCache: true}
	after.Windows = []usagestore.Window{{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.10, ResetAt: &reset, ObservedAt: afterAt, Source: "registry"}}
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
	dbPath, t0, _, before := quotaOutcomeFixture(t, 0)
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
	dbPath, t0, reset, _ := quotaOutcomeFixture(t, 0)
	afterAt := t0.Add(30 * time.Second)
	after := usagestore.TurnQuotaBoundary{SessionID: "s-out", Turn: 1, Boundary: "after", Provider: "claude", CapturedAt: afterAt, Source: "registry", Freshness: "live", HasCache: true}
	after.Windows = []usagestore.Window{{Provider: "claude", Name: "Weekly (7-day)", UsedFraction: 0.12, ResetAt: &reset, ObservedAt: afterAt, Source: "registry"}}
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
