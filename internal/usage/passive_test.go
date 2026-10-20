package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestPassiveWindowsPreserveClaudeFractionsAndReset(t *testing.T) {
	now := time.Now().UTC()
	data := []byte(`{"rate_limits":{"five_hour":{"used_percentage":0},"seven_day":{"used_percentage":100,"reset_at":"2000-01-01T00:00:00Z"}}}`)
	windows, err := passiveWindows("claude", data, now)
	if err != nil || len(windows) != 2 {
		t.Fatalf("windows=%+v err=%v", windows, err)
	}
	if windows[0].UsedFraction != 0 || windows[1].UsedFraction != 1 {
		t.Fatalf("fractions = %v, %v", windows[0].UsedFraction, windows[1].UsedFraction)
	}
	if windows[1].ResetAt == nil || !windows[1].ResetAt.Before(now) {
		t.Fatalf("past reset = %v", windows[1].ResetAt)
	}
}

func TestPassiveWindowsAcceptPartialAndAGYQuota(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		provider, json string
		want           float64
	}{
		{"claude", `{"rate_limits":{"five_hour":{"used_percentage":37.125}}}`, .37125},
		{"agy", `{"model":{"id":"gemini-pro"},"quota":{"gemini-weekly":{"remaining_fraction":0.7592765}}}`, .2407235},
		{"claude", `{"rate_limits":{"seven_day":{}}}`, -1},
	} {
		got, err := passiveWindows(tc.provider, []byte(tc.json), now)
		if err != nil {
			t.Fatal(err)
		}
		if tc.want < 0 {
			if len(got) != 0 {
				t.Errorf("partial quota produced %+v", got)
			}
			continue
		}
		if len(got) != 1 || math.Abs(got[0].UsedFraction-tc.want) > 1e-12 {
			t.Errorf("%s windows=%+v want fraction=%v", tc.provider, got, tc.want)
		}
	}
}

func TestStatuslineIngestionWritesClaudeQuotaObservation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "claude-statusline.sqlite")
	t.Cleanup(WaitObservations)
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := usagestore.EnsureSchema(context.Background(), func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	sample := []byte(`{"hook_event_name":"Status","session_id":"session-safe","workspace":{"current_dir":"/work"},"rate_limits":{"five_hour":{"used_percentage":29.125,"resets_at":"2026-10-01T00:00:00Z"},"seven_day":{"used_percentage":41.5}}}`)
	ObserveStatuslineAt(context.Background(), "claude", sample, dbPath)
	store, err = usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var source, key string
	var fraction float64
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		err = store.QueryRow(context.Background(), `SELECT o.source,q.window_key,q.used_fraction FROM usage_observations o JOIN quota_windows q ON q.observation_id=o.id WHERE o.provider='claude' ORDER BY o.id DESC LIMIT 1`).Scan(&source, &key, &fraction)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if source != "statusline" || key != "five_hour" || fraction != .29125 {
		t.Fatalf("stored Claude statusline row = source %q key %q fraction %.8f", source, key, fraction)
	}
}

func TestPassiveWriteDeduplicatesAndBestReadingUsesPrecedence(t *testing.T) {
	store, err := usagestore.Open(filepath.Join(t.TempDir(), "u.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, q string) error { return store.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	api := usagestore.Window{Provider: "claude", Key: "five_hour", Name: "5-hour", Source: "claude-api", Freshness: "fresh", UsedFraction: .42, ObservedAt: now.Add(-time.Second)}
	status := api
	status.Source = "statusline"
	status.UsedFraction = .37125
	status.ObservedAt = now
	if err := store.WriteCurrent(ctx, now, []usagestore.Window{api}); err != nil {
		t.Fatal(err)
	}
	if err := store.WritePassive(ctx, 30*time.Second, []usagestore.Window{status}); err != nil {
		t.Fatal(err)
	}
	status.ObservedAt = now.Add(time.Second)
	if err := store.WritePassive(ctx, 30*time.Second, []usagestore.Window{status}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.CurrentFor(ctx, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UsedFraction != .42 || rows[0].Source != "claude-api" {
		t.Fatalf("best reading = %+v", rows)
	}
	var n int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_observations WHERE source='statusline'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("statusline observation count=%d want one (duplicate suppressed)", n)
	}
	staleAPI := usagestore.Window{Provider: "claude", Key: "seven_day", Name: "7-day", Source: "claude-api", Freshness: "stale", UsedFraction: .8, ObservedAt: now}
	freshStatus := staleAPI
	freshStatus.Source, freshStatus.Freshness, freshStatus.UsedFraction = "statusline", "fresh", .6
	freshStatus.ObservedAt = now.Add(-time.Minute)
	if err := store.WriteCurrent(ctx, now, []usagestore.Window{staleAPI}); err != nil {
		t.Fatal(err)
	}
	if err := store.WritePassive(ctx, 30*time.Second, []usagestore.Window{freshStatus}); err != nil {
		t.Fatal(err)
	}
	rows, err = store.CurrentFor(ctx, "claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Key == "seven_day" && (row.Source != "statusline" || row.UsedFraction != .6) {
			t.Fatalf("fresh statusline did not beat stale API reading: %+v", row)
		}
	}
}

func TestPassiveObservationReturnsQuicklyWhenDatabaseLocked(t *testing.T) {
	dataHome := t.TempDir()
	t.Cleanup(WaitObservations) // runs before TempDir removal (LIFO)
	if err := os.MkdirAll(filepath.Join(dataHome, "harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	blocker, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocker.Exec(`CREATE TABLE lock_probe (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	tx, err := blocker.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO lock_probe VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("HOME", t.TempDir())
	data, _ := json.Marshal(map[string]any{"rate_limits": map[string]any{"five_hour": map[string]any{"used_percentage": 12.5}}})
	started := time.Now()
	ObserveStatusline(context.Background(), "claude", data)
	if elapsed, limit := time.Since(started), 20*time.Millisecond*raceFactor; elapsed > limit {
		t.Fatalf("locked statusline observation took %s, want <=%s", elapsed, limit)
	}
	_ = tx.Rollback()
	_ = blocker.Close()
}

func TestStatuslineObservationLargeStoreCompletesWithinBudget(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "large.sqlite")
	t.Cleanup(WaitObservations) // runs before TempDir removal (LIFO)
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	observationStmt, err := tx.Prepare(`INSERT INTO usage_observations(provider,source,observed_at,freshness) VALUES('claude','claude-api',?,'fresh')`)
	if err != nil {
		tx.Rollback()
		db.Close()
		t.Fatal(err)
	}
	windowStmt, err := tx.Prepare(`INSERT INTO quota_windows(observation_id,provider,pool,window_key,window_name,used_fraction) VALUES(?,'claude','','five_hour','5-hour',0.5)`)
	if err != nil {
		observationStmt.Close()
		tx.Rollback()
		db.Close()
		t.Fatal(err)
	}
	for i := 0; i < 50000; i++ {
		observedAt := time.Unix(1_700_000_000+int64(i), 0).UTC().Format(time.RFC3339Nano)
		result, err := observationStmt.Exec(observedAt)
		if err != nil {
			windowStmt.Close()
			observationStmt.Close()
			tx.Rollback()
			db.Close()
			t.Fatal(err)
		}
		observationID, err := result.LastInsertId()
		if err != nil {
			windowStmt.Close()
			observationStmt.Close()
			tx.Rollback()
			db.Close()
			t.Fatal(err)
		}
		if _, err := windowStmt.Exec(observationID); err != nil {
			windowStmt.Close()
			observationStmt.Close()
			tx.Rollback()
			db.Close()
			t.Fatal(err)
		}
	}
	if err := windowStmt.Close(); err != nil {
		tx.Rollback()
		db.Close()
		t.Fatal(err)
	}
	if err := observationStmt.Close(); err != nil {
		tx.Rollback()
		db.Close()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	ObserveStatuslineAt(ctx, "claude", []byte(`{"rate_limits":{"five_hour":{"used_percentage":12.5}}}`), dbPath)
	if elapsed, limit := time.Since(started), 20*time.Millisecond*raceFactor; elapsed > limit {
		t.Fatalf("statusline observation on 50k-row store took %s, want <=%s", elapsed, limit)
	}
}

func TestStatuslineSkipsMigrationUntilUsageStoreCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.sqlite")
	t.Cleanup(WaitObservations)
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		store.Close()
		t.Fatal(err)
	}
	observedAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	if err := store.Exec(ctx, `INSERT INTO usage_observations(provider,source,observed_at,freshness) VALUES('claude','claude-api',?,'fresh')`, observedAt); err != nil {
		store.Close()
		t.Fatal(err)
	}
	var observationID int64
	if err := store.QueryRow(ctx, `SELECT id FROM usage_observations WHERE observed_at=?`, observedAt).Scan(&observationID); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Exec(ctx, `INSERT INTO quota_windows(observation_id,provider,pool,window_key,window_name,used_fraction) VALUES(?,'claude','','five-hour','5-hour',0.5)`, observationID); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	ObserveStatuslineAt(ctx, "claude", []byte(`{"rate_limits":{"five_hour":{"used_percentage":12.5}}}`), dbPath)
	store, err = usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var key string
	if err := store.QueryRow(ctx, `SELECT window_key FROM quota_windows WHERE id=(SELECT min(id) FROM quota_windows WHERE observation_id=?)`, observationID).Scan(&key); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if key != "five-hour" {
		store.Close()
		t.Fatalf("statusline migrated old key to %q", key)
	}
	var migrationTable int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='usage_store_migrations'`).Scan(&migrationTable); err != nil {
		store.Close()
		t.Fatal(err)
	}
	var migrated int
	if migrationTable > 0 {
		if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name='normalized-window-keys-v4'`).Scan(&migrated); err != nil {
			store.Close()
			t.Fatal(err)
		}
	}
	if migrated != 0 {
		store.Close()
		t.Fatalf("statusline migration marker count = %d, want 0", migrated)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var observations int
		err := store.QueryRow(ctx, `SELECT count(*) FROM usage_observations WHERE source='statusline'`).Scan(&observations)
		if err == nil && observations > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if err = InitializeUsageStore(ctx, t.TempDir(), dbPath); err != nil {
		t.Fatalf("collector store initialization: %v", err)
	}
	store, err = usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.QueryRow(ctx, `SELECT window_key FROM quota_windows WHERE observation_id=?`, observationID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if want := usagestore.NormalizeWindowKey("claude", "", "5-hour"); key != want {
		t.Fatalf("usage command migrated key to %q, want %q", key, want)
	}
}
