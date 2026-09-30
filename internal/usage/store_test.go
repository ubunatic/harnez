package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/usagestore"
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

func TestCompactProjectionFailureShowsEveryProviderAndLogs(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprint(populated), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("DEBUG", "") // projection failures are always diagnostic
			summary := UsageSummary{Timestamp: time.Now()}
			if populated {
				summary.Agents = []AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: true, Session: &QuotaWindow{Name: "Session", UsedPercent: 37}}}
			}
			blocked := filepath.Join(home, "blocked")
			if err := os.WriteFile(blocked, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := StoreCompactSummaryAt(context.Background(), home, summary, filepath.Join(blocked, "usage.sqlite"))
			if err == nil {
				t.Fatal("expected projection failure")
			}
			if len(got.Agents) != 3 {
				t.Fatalf("providers missing or duplicated: %+v", got.Agents)
			}
			rows := strings.Join(allUsageLines(got, 240, false), "\n")
			for _, name := range []string{"Claude Code", "OpenAI Codex", "Antigravity (AGY)"} {
				if !strings.Contains(rows, name) {
					t.Errorf("provider %q silently dropped from %s", name, rows)
				}
			}
			for _, agent := range got.Agents {
				if agent.QuotaFetchError == "" || !agent.HasUsageData() {
					t.Errorf("provider has no visible error: %+v", agent)
				}
			}
			if !strings.Contains(rows, "unavailable (store projection failed:") {
				t.Errorf("no error row in %s", rows)
			}
			log, readErr := os.ReadFile(filepath.Join(home, ".harnez", "debug.log"))
			if readErr != nil || !strings.Contains(string(log), err.Error()) || !strings.Contains(string(log), "usage compact projection failed") {
				t.Fatalf("missing debug diagnostic: %s, %v", log, readErr)
			}
			if populated && (summary.Agents[0].QuotaFetchError != "" || got.Agents[0].Session.UsedPercent != 37) {
				t.Fatal("fallback mutated input or lost collected quota")
			}
		})
	}
}

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

func TestCompactRenderingIncludesClaudeCodexAndAGYFromLabeledWriters(t *testing.T) {
	isolateUsageTestStorage(t)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "all-providers.sqlite")
	home := t.TempDir()
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, q string) error { return store.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	reset := at.Add(5 * time.Hour)
	agents := []AgentUsage{
		{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true, Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 8, ResetAt: &reset}, Weekly: &QuotaWindow{Name: "Weekly (7-day)", UsedPercent: 66, ResetAt: &reset}},
		{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true, Session: &QuotaWindow{Name: "5-Hour", UsedPercent: 7, ResetAt: &reset}, Weekly: &QuotaWindow{Name: "Weekly", UsedPercent: 65, ResetAt: &reset}},
		{AgentID: "agy", Name: "Antigravity (AGY)", Installed: true, Authenticated: true, ModelGroups: []ModelGroup{{Name: "Gemini Models", Windows: []QuotaWindow{{Name: "Five Hour Limit Remaining", UsedPercent: 11, ResetAt: &reset}, {Name: "Weekly Limit Remaining", UsedPercent: 22, ResetAt: &reset}}}}},
	}
	for _, agent := range agents {
		if err := importAgent(store, ctx, agent.AgentID, "registry", at, agent); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate all providers arriving in one collect-all pass with an identical
	// observation timestamp; uniqueness is provider-scoped, not global.
	for _, agent := range agents {
		if err := importAgent(store, ctx, agent.AgentID, "collect-all", at.Add(time.Second), agent); err != nil {
			t.Fatal(err)
		}
	}
	// A token counter is not a quota window and must never be projected into
	// the compact quota rows, even though its used_fraction is zero.
	if err := store.WriteCurrent(ctx, at.Add(2*time.Second), []usagestore.Window{
		{Provider: "claude", Pool: "tokens", Key: "cumulative", Name: "tokens", Source: "turn-capture", Freshness: "fresh"},
		{Provider: "codex", Pool: "tokens", Key: "cumulative", Name: "tokens", Source: "turn-capture", Freshness: "fresh"},
	}); err != nil {
		t.Fatal(err)
	}
	// A newer low-priority observation must beat an older registry snapshot in
	// the compact-specific latest view.
	newer := at.Add(3 * time.Second)
	if err := importAgent(store, ctx, "claude", "statusline", newer, AgentUsage{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true, Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 9, ResetAt: &reset}}); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"claude", "codex", "agy"} {
		if err := importAgent(store, ctx, provider, "collect-all", at.Add(time.Second), AgentUsage{AgentID: provider, Name: provider, Installed: true, Authenticated: true}); err != nil {
			t.Fatal(err)
		}
	}
	var mismatch int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id WHERE q.provider<>o.provider`).Scan(&mismatch); err != nil {
		t.Fatal(err)
	}
	if mismatch != 0 {
		t.Fatalf("cross-provider observation links = %d", mismatch)
	}
	for _, provider := range []string{"claude", "codex", "agy"} {
		windows, err := store.CurrentFor(ctx, provider)
		if err != nil {
			t.Fatal(err)
		}
		if len(windows) == 0 {
			t.Fatalf("no current windows for %s", provider)
		}
	}
	// Simulate live collection returning only AGY this cycle; Claude and Codex
	// must still be restored from their latest durable provider windows.
	summary := UsageSummary{Timestamp: at, Agents: []AgentUsage{agents[2]}}
	projected, err := StoreCompactSummaryAt(ctx, home, summary, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var compact strings.Builder
	for _, line := range allUsageLinesAt(projected, 100, false, at, time.Minute) {
		compact.WriteString(stripANSI(line))
		compact.WriteByte('\n')
	}
	for _, want := range []string{"Claude Code", "OpenAI Codex", "Gemini"} {
		if !strings.Contains(compact.String(), want) {
			t.Fatalf("compact output missed %q:\n%s", want, compact.String())
		}
	}
	if strings.Contains(compact.String(), "tokens") || strings.Contains(compact.String(), "0%") {
		t.Fatalf("compact output included a token pseudo-window:\n%s", compact.String())
	}
	var claudeSession *QuotaWindow
	for _, agent := range projected.Agents {
		if agent.AgentID == "claude" {
			claudeSession = agent.Session
		}
	}
	if claudeSession == nil || claudeSession.UsedPercent != 9 {
		t.Fatalf("compact projection did not choose newest Claude session: %+v", claudeSession)
	}
}

func TestCompactAllUsageDisplaysCodexFetchErrorWithoutWindows(t *testing.T) {
	got := allUsageLinesAt(UsageSummary{Agents: []AgentUsage{{AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true, QuotaFetchError: "HTTP 401"}}}, 100, false, time.Now(), time.Minute)
	if len(got) != 1 || !strings.Contains(stripANSI(got[0]), "OpenAI Codex") || !strings.Contains(stripANSI(got[0]), "unavailable") {
		t.Fatalf("compact error row = %q", got)
	}
}
