package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestHistoryImportSkipsUnchangedFilesButConsumesChanges(t *testing.T) {
	isolateUsageTestStorage(t)
	ctx := context.Background()
	home := t.TempDir()
	dir := HistoryDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fixture.jsonl")
	entry := HistoryEntry{Hostname: "fixture", UsageSummary: UsageSummary{Timestamp: time.Now()}}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := usagestore.Open(filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		t.Fatal(err)
	}
	if err := importUsageSummaryHistory(store, ctx, home); err != nil {
		t.Fatal(err)
	}
	// Even idempotent INSERT/UPDATE statements previously rewrote every row.
	// A rejecting trigger proves the second read performs no history writes.
	if err := store.Exec(ctx, `CREATE TRIGGER reject_replay BEFORE INSERT ON usage_summary_observations BEGIN SELECT RAISE(ABORT,'unexpected replay'); END`); err != nil {
		t.Fatal(err)
	}
	if err := importUsageSummaryHistory(store, ctx, home); err != nil {
		t.Fatalf("unchanged history was replayed: %v", err)
	}
	if err := store.Exec(ctx, `DROP TRIGGER reject_replay`); err != nil {
		t.Fatal(err)
	}
	entry.Timestamp = entry.Timestamp.Add(time.Minute)
	more, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(append(data, '\n'), append(more, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := importUsageSummaryHistory(store, ctx, home); err != nil {
		t.Fatal(err)
	}
	rows, err := store.UsageSummaries(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("changed file lost or duplicated records: %d, %v", len(rows), err)
	}
}

func TestCompatibilityImporterBackfillsSnapshotOnce(t *testing.T) {
	isolateUsageTestStorage(t)
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	stateDir := StateDir(home)
	if err := WriteAgentSnapshot(stateDir, "claude", AgentUsage{AgentID: "claude", Session: &QuotaWindow{Name: "5-hour", UsedPercent: 33.333, RemainingPercent: 66.667}}); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "store.sqlite")
	if err := InitializeUsageStore(context.Background(), home, dbPath); err != nil {
		t.Fatal(err)
	}
	_, got, err := ProviderSnapshotFromStoreAt(context.Background(), dbPath, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.Session == nil || got.Session.UsedPercent != 33.333 {
		t.Fatalf("imported snapshot reading = %+v", got.Session)
	}
}

func TestCompactAllUsageDisplaysCodexFetchErrorWithoutWindows(t *testing.T) {
	got := allUsageLinesAt(UsageSummary{Agents: []AgentUsage{{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true, QuotaFetchError: "HTTP 401"}}}, 100, false, time.Now(), time.Minute)
	if len(got) != 1 || !strings.Contains(stripANSI(got[0]), "OpenAI Codex") || !strings.Contains(stripANSI(got[0]), "unavailable") {
		t.Fatalf("compact error row = %q", got)
	}
}

// The collector refreshes the current snapshot every tick while history remains throttled.
func TestCollectorSummaryRefreshesCurrentAndThrottlesHistory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	home := t.TempDir()
	if err := InitializeUsageStore(context.Background(), home, dbPath); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{start, start.Add(time.Minute), start.Add(16 * time.Minute)} {
		summary := UsageSummary{Timestamp: at, Agents: []AgentUsage{{AgentID: "claude", Installed: true}}}
		if err := PersistUsageSummary(context.Background(), home, dbPath, summary); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := ReadStoredSummary(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Timestamp.Equal(start.Add(16 * time.Minute)) {
		t.Fatalf("current summary timestamp = %s, want latest collector write %s", stored.Timestamp, start.Add(16*time.Minute))
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	history, err := store.UsageSummaries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || !history[0].ObservedAt.Equal(start) || !history[1].ObservedAt.Equal(start.Add(16*time.Minute)) {
		t.Fatalf("history snapshots = %+v, want start and start+16m", history)
	}
}
