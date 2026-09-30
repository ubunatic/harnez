package usage

import (
	"context"
	"database/sql"
	"path/filepath"
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
