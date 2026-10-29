package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

func TestUsageHistoryFromStoreImportsLegacyHostRecords(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	dataHome := filepath.Join(base, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	dir := HistoryDir(home)
	writeJSONLine(t, filepath.Join(dir, "host.jsonl"), HistoryEntry{Hostname: "host", UsageSummary: UsageSummary{Timestamp: at, Agents: []AgentUsage{{AgentID: "claude", Session: &QuotaWindow{Name: "5h", UsedPercent: 12.5, RemainingPercent: 87.5}}}}})
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	for range 2 {
		entries, err := UsageHistoryFromStore(context.Background(), home, dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Hostname != "host" || entries[0].Agents[0].Session.UsedPercent != 12.5 {
			t.Fatalf("store history = %+v", entries)
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var count int
	if err := store.QueryRow(context.Background(), `SELECT count(*) FROM usage_summary_observations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("summary rows = %d, want 1", count)
	}
}

func TestAppendHistoryUsesStoreAndGeneratesCompatibilityMirror(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	dataHome := filepath.Join(base, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	summary := UsageSummary{Timestamp: at, Agents: []AgentUsage{{AgentID: "codex", Weekly: &QuotaWindow{Name: "weekly", UsedPercent: 31}}}}
	dir := HistoryDir(home)
	if err := AppendHistory(dir, summary); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Agents[0].Weekly.UsedPercent != 31 {
		t.Fatalf("ReadHistory() = %+v", entries)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("history mirror files = %v", files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var mirrored HistoryEntry
	if err := json.Unmarshal(data[:len(data)-1], &mirrored); err != nil {
		t.Fatal(err)
	}
	if mirrored.Timestamp != at || mirrored.Agents[0].Weekly.UsedPercent != 31 {
		t.Fatalf("generated mirror = %+v", mirrored)
	}
}

func TestAgentSnapshotReaderUsesStoreAfterCompatibilityImport(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	dataHome := filepath.Join(base, "data")
	stateHome := filepath.Join(base, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("XDG_STATE_HOME", stateHome)
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	want := AgentSnapshot{FetchedAt: at, Usage: AgentUsage{AgentID: "agy", ModelGroups: []ModelGroup{{Name: "Gemini Models", Windows: []QuotaWindow{{Name: "Weekly Limit Remaining", UsedPercent: 19, RemainingPercent: 81}}}}}}
	if err := writeAgentSnapshotFile(StateDir(home), "agy", want.Usage); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	got, err := readAgentSnapshotFromStore(context.Background(), dbPath, "agy")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Usage.AgentID != "agy" || len(got.Usage.ModelGroups) != 1 || got.Usage.ModelGroups[0].Windows[0].UsedPercent != 19 {
		t.Fatalf("store snapshot = %+v", got)
	}
}

func TestProviderSnapshotAndQuotaHistoryMirrorUseStore(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	dataHome := filepath.Join(base, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := usagestore.EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return store.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	window := QuotaWindow{Name: "5h", UsedPercent: 23.5, RemainingPercent: 76.5}
	if err := store.WriteCurrent(context.Background(), at, AgentUsageToStoreWindows(AgentUsage{AgentID: "claude", Session: &window}, at)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	gotAt, got, err := ProviderSnapshotFromStoreAt(context.Background(), dbPath, "claude")
	if err != nil || !gotAt.Equal(at) || got.Session == nil || got.Session.UsedPercent != 23.5 {
		t.Fatalf("provider snapshot at=%s usage=%+v err=%v", gotAt, got, err)
	}
	entry := QuotaHistoryEntry{Timestamp: at.Add(2 * time.Minute), Agent: "claude", Window: "5h", UsedPercent: 24, RemainingPercent: 76}
	if err := AppendQuotaHistoryWithThrottle(HistoryDir(home), []QuotaHistoryEntry{entry}, time.Minute, entry.Timestamp); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadQuotaHistory(HistoryDir(home))
	if err != nil || len(rows) != 2 || rows[0].Window != "5h" || rows[0].UsedPercent != 24 || rows[1].UsedPercent != 24 {
		t.Fatalf("store quota history = %+v err=%v", rows, err)
	}
	data, err := os.ReadFile(QuotaHistoryPath(HistoryDir(home)))
	if err != nil {
		t.Fatal(err)
	}
	var mirrorRows []QuotaHistoryEntry
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var row QuotaHistoryEntry
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		mirrorRows = append(mirrorRows, row)
	}
	if len(mirrorRows) != 2 || mirrorRows[0].Window != "5h" || mirrorRows[0].UsedPercent != 24 || mirrorRows[1].UsedPercent != 24 {
		t.Fatalf("quota history mirror = %+v", mirrorRows)
	}
}

// Issue 735: the same history file in the XDG dir and the legacy
// ~/.claude/harnez dir must export each usage point once.
func TestUsageHistoryFromStoreImportsDuplicateDirectoriesOnce(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	dataHome := filepath.Join(base, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	at := time.Date(2026, 9, 16, 19, 3, 41, 0, time.UTC)
	entry := HistoryEntry{Hostname: "x600", UsageSummary: UsageSummary{Timestamp: at, Agents: []AgentUsage{{AgentID: "claude"}, {AgentID: "codex"}}}}
	writeJSONLine(t, filepath.Join(HistoryDir(home), "x600.jsonl"), entry)
	writeJSONLine(t, filepath.Join(home, ".claude", "harnez", historyDirName, "x600.jsonl"), entry)
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	entries, err := UsageHistoryFromStore(context.Background(), home, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	exp := BuildUsageExport(entries, at)
	seen := map[string]bool{}
	for _, p := range exp.Points {
		key := p.Timestamp.String() + "|" + p.Hostname + "|" + p.AgentID
		if seen[key] {
			t.Fatalf("duplicate usage point %s in %+v", key, exp.Points)
		}
		seen[key] = true
	}
	if len(exp.Points) != 2 {
		t.Fatalf("points = %d, want 2", len(exp.Points))
	}
}

func TestApplyStoredWindowsDropsStatuslineDefaultWhenModelGroupsExist(t *testing.T) {
	now := time.Date(2026, 10, 25, 19, 0, 0, 0, time.UTC)
	expired := now.Add(-15 * 24 * time.Hour)
	future := now.Add(time.Hour)
	agent := AgentUsage{AgentID: "agy", Name: "Antigravity (AGY)"}
	applyStoredWindows(&agent, []usagestore.Window{
		{Provider: "agy", Key: "default", Name: "default", Source: "statusline", Freshness: "stale", UsedFraction: .95, ResetAt: &expired, ObservedAt: expired},
		{Provider: "agy", Pool: "Gemini Models", Key: "weekly", Name: "Weekly", Source: "agy-meter", Freshness: "fresh", UsedFraction: .24, ResetAt: &future, ObservedAt: now},
	}, now)
	if agent.Session != nil {
		t.Fatalf("expired statusline default session survived alongside model groups: %+v", agent.Session)
	}
	if len(agent.ModelGroups) != 1 || agent.ModelGroups[0].Windows[0].UsedPercent != 24 {
		t.Fatalf("model groups = %+v, want the fresh Gemini window", agent.ModelGroups)
	}
}

func TestApplyStoredWindowsKeepsMonthlyInLongSlot(t *testing.T) {
	for _, name := range []string{"Monthly", "Monthly (stale)", "30-day", "30 day"} {
		var agent AgentUsage
		key := usagestore.NormalizeWindowKey("codex", "", name)
		applyStoredWindows(&agent, []usagestore.Window{{Key: key, Name: name, Source: "test", UsedFraction: 0.1}}, time.Now())
		if agent.Weekly == nil || agent.Session != nil {
			t.Errorf("%q (key %q): Weekly = %+v, Session = %+v; want the long slot only", name, key, agent.Weekly, agent.Session)
		}
	}
}
