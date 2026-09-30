package usagestore

import (
	"context"
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndCurrentIdempotentAndFractional(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	items := []Window{{Provider: "claude", Key: "5h", Name: "5h", Source: "collect-all", Freshness: "fresh", UsedFraction: .43125, ObservedAt: at}}
	for range 2 {
		if err := s.WriteCurrent(context.Background(), at, items); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UsedFraction != .43125 || got[0].Freshness != "fresh" {
		t.Fatalf("Current() = %+v", got)
	}
}

func TestEnsureSchemaAddsTypedWindowColumnsToLegacyDatabase(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "legacy.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, statement := range []string{
		`CREATE TABLE usage_observations (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, source TEXT NOT NULL, observed_at TEXT NOT NULL, freshness TEXT NOT NULL, payload_version TEXT NOT NULL DEFAULT '', UNIQUE(provider,source,observed_at))`,
		`CREATE TABLE quota_windows (id INTEGER PRIMARY KEY AUTOINCREMENT, observation_id INTEGER NOT NULL, provider TEXT NOT NULL, pool TEXT NOT NULL DEFAULT '', window_key TEXT NOT NULL, window_name TEXT NOT NULL DEFAULT '', used_fraction REAL NOT NULL, reset_at TEXT, UNIQUE(observation_id,pool,window_key))`,
	} {
		if err := s.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnsureSchema(ctx, func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	input := int64(17)
	if err := s.WriteCurrent(ctx, time.Now(), []Window{{Provider: "claude", Key: "tokens", Name: "tokens", Source: "collect-all", Freshness: "fresh", InputTokens: &input}}); err != nil {
		t.Fatalf("write after legacy schema migration: %v", err)
	}
}

func TestWriteLoadObservationPersistsNormalizedPayload(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	if err := s.WriteLoadObservation(context.Background(), "remote-load", at, map[string]any{"cpu_percent": 12.5, "cores": 8}); err != nil {
		t.Fatal(err)
	}
	var provider, observed, payload string
	if err := s.QueryRow(context.Background(), `SELECT provider,observed_at,payload_json FROM usage_load_observations`).Scan(&provider, &observed, &payload); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	if provider != "remote-load" || observed != at.Format(time.RFC3339Nano) || decoded["cpu_percent"] != 12.5 {
		t.Fatalf("load row provider=%q observed=%q payload=%s", provider, observed, payload)
	}
}

func TestCurrentKeepsLatestAndStaleMetadata(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, w := range []Window{{Provider: "codex", Key: "weekly", Name: "Weekly", Source: "history", Freshness: "stale", UsedFraction: .99, ObservedAt: base}, {Provider: "codex", Key: "weekly", Name: "Weekly", Source: "collect-all", Freshness: "fresh", UsedFraction: .25, ObservedAt: base.Add(time.Minute)}} {
		if err := s.WriteCurrent(context.Background(), base, []Window{w}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UsedFraction != .25 || got[0].Source != "collect-all" || got[0].Freshness != "fresh" {
		t.Fatalf("Current() = %+v", got)
	}
}

func TestMigrateStableWindowKeysSeparatesStalenessFromLabel(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := EnsureSchema(ctx, func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := s.WriteCurrent(ctx, at, []Window{{Provider: "claude", Key: "5-Hour (stale)", Name: "5-Hour (stale)", Source: "history", Freshness: "fresh", UsedFraction: .23, ObservedAt: at}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateStableWindowKeys(ctx); err != nil {
		t.Fatal(err)
	}
	var key, name, freshness string
	if err := s.QueryRow(ctx, `SELECT q.window_key,q.window_name,o.freshness FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id`).Scan(&key, &name, &freshness); err != nil {
		t.Fatal(err)
	}
	if key != "five_hour" || name != "5-Hour" || freshness != "stale" {
		t.Fatalf("migrated window=%q name=%q freshness=%q", key, name, freshness)
	}
}

func TestNormalizeWindowKeyHandlesLegacyProviderAliasesAndPools(t *testing.T) {
	for _, item := range []struct {
		provider, pool, name, want string
	}{
		{"claude", "", "5h", "five_hour"},
		{"claude", "", "five_hour_limit_remaining", "five_hour"},
		{"codex", "", "7-day limit remaining", "weekly"},
		{"agy", "Gemini Models", "Weekly Limit Remaining", "weekly"},
		{"agy", "Claude and GPT models", "5h", "five_hour"},
		{"claude", "", "Spend Limit", "spend_limit"},
		{"codex", "", "cumulative", "cumulative"},
	} {
		if got := NormalizeWindowKey(item.provider, item.pool, item.name); got != item.want {
			t.Errorf("NormalizeWindowKey(%q, %q, %q) = %q, want %q", item.provider, item.pool, item.name, got, item.want)
		}
	}
}

func TestMigrateStableWindowKeysArchivesCollisionsAndPreservesOriginal(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "legacy-keys.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := EnsureSchema(ctx, func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if err := s.Exec(ctx, `INSERT INTO usage_observations(provider,source,observed_at,freshness) VALUES('agy','legacy','`+at+`','fresh')`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		key, name string
	}{
		{"5h", "5h"},
		{"five_hour_limit_remaining", "Five Hour Limit Remaining"},
	} {
		if err := s.Exec(ctx, `INSERT INTO quota_windows(observation_id,provider,pool,window_key,window_name,used_fraction) VALUES(1,'agy','Gemini Models',?,?,0.25)`, row.key, row.name); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Exec(ctx, `INSERT INTO usage_observations(provider,source,observed_at,freshness) VALUES('codex','legacy','`+at+`','fresh')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Exec(ctx, `INSERT INTO quota_windows(observation_id,provider,pool,window_key,window_name,used_fraction) VALUES(2,'codex','', 'weekly_limit_remaining','7-day limit remaining',0.5)`); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateStableWindowKeys(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.QueryRow(ctx, `SELECT count(*) FROM quota_windows WHERE window_key IN ('5h','five_hour_limit_remaining','weekly_limit_remaining') OR window_key LIKE '%_duplicate_%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("legacy keys remain in reader table: %d", count)
	}
	var fiveKey, fiveLegacy string
	if err := s.QueryRow(ctx, `SELECT window_key,legacy_window_key FROM quota_windows WHERE provider='agy'`).Scan(&fiveKey, &fiveLegacy); err != nil {
		t.Fatal(err)
	}
	if fiveKey != "five_hour" || fiveLegacy != "5h" {
		t.Fatalf("five-hour key provenance = %q / %q", fiveKey, fiveLegacy)
	}
	if err := s.QueryRow(ctx, `SELECT count(*) FROM usage_window_key_migration_archive WHERE original_window_key='five_hour_limit_remaining'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("archived collision count = %d, want 1", count)
	}
	var weeklyKey, weeklyName string
	if err := s.QueryRow(ctx, `SELECT window_key,window_name FROM quota_windows WHERE provider='codex'`).Scan(&weeklyKey, &weeklyName); err != nil {
		t.Fatal(err)
	}
	if weeklyKey != "weekly" || weeklyName != "7-day limit remaining" {
		t.Fatalf("weekly key/name = %q / %q", weeklyKey, weeklyName)
	}
	if err := s.MigrateStableWindowKeys(ctx); err != nil {
		t.Fatalf("second migration call: %v", err)
	}
}

func TestTurnQuotaBoundaryPairsOnlyOrderedNonResetWindows(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "turns.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := EnsureSchema(ctx, func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	reset := start.Add(5 * time.Hour)
	for _, item := range []struct {
		boundary string
		at       time.Time
		used     float64
		reset    *time.Time
	}{
		{boundary: "before", at: start, used: .21, reset: &reset},
		{boundary: "after", at: start.Add(time.Minute), used: .33, reset: &reset},
	} {
		_, err := s.WriteTurnQuotaBoundary(ctx, TurnQuotaBoundary{
			SessionID: "session-1", Turn: 1, Boundary: item.boundary, Provider: "claude", CapturedAt: item.at,
			Windows: []Window{{Provider: "claude", Source: "claude-api", Freshness: "fresh", Pool: "", Key: "five_hour", Name: "5-hour", UsedFraction: item.used, ResetAt: item.reset, ObservedAt: item.at}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PairTurnQuotaDeltas(ctx, "session-1", 1, "claude"); err != nil {
		t.Fatal(err)
	}
	deltas, err := s.TurnQuotaDeltas(ctx, "session-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 || deltas[0].WindowKey != "five_hour" || math.Abs(deltas[0].Delta-.12) > 1e-9 || deltas[0].BeforeSource != "claude-api" || deltas[0].AfterSource != "claude-api" {
		t.Fatalf("paired deltas = %+v", deltas)
	}

	newReset := reset.Add(5 * time.Hour)
	for _, item := range []struct {
		boundary string
		at       time.Time
		used     float64
		reset    *time.Time
	}{
		{boundary: "before", at: start.Add(2 * time.Minute), used: .9, reset: &reset},
		{boundary: "after", at: start.Add(3 * time.Minute), used: .1, reset: &newReset},
	} {
		_, err := s.WriteTurnQuotaBoundary(ctx, TurnQuotaBoundary{
			SessionID: "session-1", Turn: 2, Boundary: item.boundary, Provider: "claude", CapturedAt: item.at,
			Windows: []Window{{Provider: "claude", Source: "claude-api", Freshness: "fresh", Key: "five_hour", Name: "5-hour", UsedFraction: item.used, ResetAt: item.reset, ObservedAt: item.at}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PairTurnQuotaDeltas(ctx, "session-1", 2, "claude"); err != nil {
		t.Fatal(err)
	}
	deltas, err = s.TurnQuotaDeltas(ctx, "session-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 0 {
		t.Fatalf("reset-crossing deltas = %+v, want none", deltas)
	}
}

func TestWriteTurnTokenUsageStoresDimensionsAndQuality(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "tokens.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := EnsureSchema(ctx, func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	input, cached, output := int64(41), int64(12), int64(7)
	err = s.WriteTurnTokenUsage(ctx, TurnTokenUsage{
		SessionID: "session-1", Turn: 2, Provider: "codex", CounterKind: "delta",
		InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output,
		InputQuality: "measured", CachedInputQuality: "measured", OutputQuality: "measured", ReasoningQuality: "unknown",
		ObservedAt: time.Date(2026, 9, 30, 12, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	var gotInput, gotCached, gotOutput int64
	var inputQuality, reasoningQuality string
	if err := s.QueryRow(ctx, `SELECT input_tokens,cached_input_tokens,output_tokens,input_quality,reasoning_quality FROM turn_token_usage WHERE session_id='session-1' AND turn=2 AND counter_kind='delta'`).Scan(&gotInput, &gotCached, &gotOutput, &inputQuality, &reasoningQuality); err != nil {
		t.Fatal(err)
	}
	if gotInput != input || gotCached != cached || gotOutput != output || inputQuality != "measured" || reasoningQuality != "unknown" {
		t.Fatalf("token row = input %d cached %d output %d qualities %q/%q", gotInput, gotCached, gotOutput, inputQuality, reasoningQuality)
	}
}
