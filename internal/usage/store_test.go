package usage

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestStoreCompactSummaryWritesAndReadsFreshReading(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "usage.sqlite")
	summary := UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude", Installed: true, LastRefreshed: time.Now(), Session: &QuotaWindow{Name: "5-hour", UsedPercent: 42.125, RemainingPercent: 57.875}}}}
	got, err := StoreCompactSummaryAt(context.Background(), t.TempDir(), summary, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Agents[0].Session == nil || got.Agents[0].Session.UsedPercent != 42.125 {
		t.Fatalf("reading = %+v", got.Agents[0].Session)
	}
	telemetryDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer telemetryDB.Close()
	var n int
	if err := telemetryDB.QueryRow(`SELECT count(*) FROM usage_observations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no observations persisted")
	}
}

func TestStoreCompactSummaryUnavailableFallsBack(t *testing.T) {
	summary := UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude", Session: &QuotaWindow{Name: "5-hour", UsedPercent: 37}}}}
	got, err := StoreCompactSummaryAt(context.Background(), t.TempDir(), summary, "/dev/null/usage.sqlite")
	if err == nil {
		t.Fatal("expected database path error")
	}
	if got.Agents[0].Session == nil || got.Agents[0].Session.UsedPercent != 37 {
		t.Fatalf("fallback summary lost current value: %+v", got)
	}
}

func TestCompatibilityImporterBackfillsSnapshotOnce(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, ".local", "state", "harnez", "agents", "usage")
	if err := WriteAgentSnapshot(stateDir, "claude", AgentUsage{AgentID: "claude", Session: &QuotaWindow{Name: "5-hour", UsedPercent: 33.333, RemainingPercent: 66.667}}); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "store.sqlite")
	summary := UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "claude", Installed: true}}}
	got, err := StoreCompactSummaryAt(context.Background(), home, summary, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Agents[0].Session == nil || got.Agents[0].Session.UsedPercent != 33.333 {
		t.Fatalf("imported snapshot reading = %+v", got.Agents[0].Session)
	}
}

func TestCompactCollectionPersistsChangedReadingsAndRendersLatest(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "compact.sqlite")
	home := t.TempDir()
	firstAt := time.Now().Add(-time.Minute)
	first := UsageSummary{Timestamp: firstAt, Agents: []AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true, LastRefreshed: firstAt, Session: &QuotaWindow{Name: "5-Hour (stale)", UsedPercent: 23}}}}
	if _, err := StoreCompactSummaryAt(context.Background(), home, first, dbPath); err != nil {
		t.Fatal(err)
	}
	secondAt := time.Now()
	second := UsageSummary{Timestamp: secondAt, Agents: []AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true, LastRefreshed: secondAt, Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 29}}}}
	got, err := StoreCompactSummaryAt(context.Background(), home, second, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Agents[0].Session == nil || math.Abs(got.Agents[0].Session.UsedPercent-29) > 1e-9 {
		t.Fatalf("latest compact reading = %+v", got.Agents[0].Session)
	}
	if rendered := RenderText(got); !strings.Contains(rendered, "29.0% used") {
		t.Fatalf("compact render missed new value:\n%s", rendered)
	}
	storeDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer storeDB.Close()
	var obs, windows int
	if err := storeDB.QueryRow(`SELECT count(*) FROM usage_observations WHERE provider='claude' AND source='collect-all'`).Scan(&obs); err != nil {
		t.Fatal(err)
	}
	if err := storeDB.QueryRow(`SELECT count(*) FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id WHERE q.provider='claude' AND o.source='collect-all'`).Scan(&windows); err != nil {
		t.Fatal(err)
	}
	if obs != 2 || windows != 2 {
		t.Fatalf("compact writes: observations=%d windows=%d, want 2 each", obs, windows)
	}
	var key, name string
	if err := storeDB.QueryRow(`SELECT window_key,window_name FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id WHERE q.provider='claude' AND o.source='collect-all' ORDER BY o.observed_at DESC LIMIT 1`).Scan(&key, &name); err != nil {
		t.Fatal(err)
	}
	if key != "five_hour" || name != "Session (5-hour)" {
		t.Fatalf("stored key/name = %q/%q", key, name)
	}
}
