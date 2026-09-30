// Package usagestore persists normalized provider quota observations.
package usagestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

type Window struct {
	Provider, Pool, Key, Name, Source, Freshness                  string
	UsedFraction                                                  float64
	InputTokens, CachedInputTokens, OutputTokens, ReasoningTokens *int64
	ResetAt                                                       *time.Time
	ObservedAt                                                    time.Time
}

func Open(path string) (*Store, error) {
	return OpenWithBusyTimeout(path, 5*time.Second)
}

// OpenWithBusyTimeout opens the store with an explicit SQLite lock wait.
func OpenWithBusyTimeout(path string, busyTimeout time.Duration) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	if busyTimeout < 0 {
		busyTimeout = 0
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)", path, busyTimeout.Milliseconds()))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Exec(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *Store) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// WriteCurrent appends a normalized observation for each provider returned by
// the existing collectors. It is safe to retry the same observation.
func (s *Store) WriteCurrent(ctx context.Context, observedAt time.Time, windows []Window) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, w := range windows {
		at := w.ObservedAt
		if at.IsZero() {
			at = observedAt
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO usage_observations(provider,source,observed_at,freshness) VALUES(?,?,?,?) ON CONFLICT(provider,source,observed_at) DO UPDATE SET freshness=excluded.freshness`, w.Provider, w.Source, at.UTC().Format(time.RFC3339Nano), w.Freshness)
		if err != nil {
			return fmt.Errorf("insert usage observation: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if id == 0 {
			if err := tx.QueryRowContext(ctx, `SELECT id FROM usage_observations WHERE provider=? AND source=? AND observed_at=?`, w.Provider, w.Source, at.UTC().Format(time.RFC3339Nano)).Scan(&id); err != nil {
				return err
			}
		}
		var reset any
		if w.ResetAt != nil {
			reset = w.ResetAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO quota_windows(observation_id,provider,pool,window_key,window_name,used_fraction,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,reset_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(observation_id,pool,window_key) DO UPDATE SET window_name=excluded.window_name,used_fraction=excluded.used_fraction,input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,output_tokens=excluded.output_tokens,reasoning_tokens=excluded.reasoning_tokens,reset_at=excluded.reset_at`, id, w.Provider, w.Pool, w.Key, w.Name, w.UsedFraction, w.InputTokens, w.CachedInputTokens, w.OutputTokens, w.ReasoningTokens, reset); err != nil {
			return fmt.Errorf("insert quota window: %w", err)
		}
	}
	return tx.Commit()
}

// WritePassive appends passive observations after suppressing identical
// readings seen within dedupeInterval.
func (s *Store) WritePassive(ctx context.Context, dedupeInterval time.Duration, windows []Window) error {
	filtered := make([]Window, 0, len(windows))
	for _, w := range windows {
		if w.ObservedAt.IsZero() {
			w.ObservedAt = time.Now()
		}
		var count int
		err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id WHERE q.provider=? AND q.pool=? AND q.window_key=? AND o.source=? AND q.used_fraction=? AND COALESCE(q.reset_at,'')=COALESCE(?, '') AND o.observed_at>=?`, w.Provider, w.Pool, w.Key, w.Source, w.UsedFraction, timeValue(w.ResetAt), w.ObservedAt.Add(-dedupeInterval).UTC().Format(time.RFC3339Nano)).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			filtered = append(filtered, w)
		}
	}
	return s.WriteCurrent(ctx, time.Now(), filtered)
}

func timeValue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// WriteLoadObservation appends a normalized CPU/GPU load sample as JSON while
// preserving the existing typed snapshot wire format.
func (s *Store) WriteLoadObservation(ctx context.Context, provider string, observedAt time.Time, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal load observation: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO usage_load_observations(provider,observed_at,payload_json) VALUES(?,?,?) ON CONFLICT(provider,observed_at) DO UPDATE SET payload_json=excluded.payload_json`, provider, observedAt.UTC().Format(time.RFC3339Nano), string(data))
	if err != nil {
		return fmt.Errorf("insert load observation: %w", err)
	}
	return nil
}

// Current returns the newest observation for each provider/pool/window.
func (s *Store) Current(ctx context.Context) ([]Window, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT q.provider,q.pool,q.window_key,q.window_name,o.source,o.freshness,q.used_fraction,q.reset_at,o.observed_at FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id JOIN (SELECT q2.provider,q2.pool,q2.window_key,MAX(o2.observed_at) observed_at FROM quota_windows q2 JOIN usage_observations o2 ON o2.id=q2.observation_id GROUP BY q2.provider,q2.pool,q2.window_key) latest ON latest.provider=q.provider AND latest.pool=q.pool AND latest.window_key=q.window_key AND latest.observed_at=o.observed_at ORDER BY q.provider,q.pool,q.window_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Window
	for rows.Next() {
		var w Window
		var reset sql.NullString
		var observed string
		if err := rows.Scan(&w.Provider, &w.Pool, &w.Key, &w.Name, &w.Source, &w.Freshness, &w.UsedFraction, &reset, &observed); err != nil {
			return nil, err
		}
		w.ObservedAt, err = time.Parse(time.RFC3339Nano, observed)
		if err != nil {
			return nil, err
		}
		if reset.Valid {
			t, e := time.Parse(time.RFC3339Nano, reset.String)
			if e != nil {
				return nil, e
			}
			w.ResetAt = &t
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// CurrentFor returns the newest observation per window for one provider,
// pool, and key. Compatibility reads use source priority for tied timestamps.
func (s *Store) CurrentFor(ctx context.Context, provider string) ([]Window, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider,pool,window_key,window_name,source,freshness,used_fraction,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,reset_at,observed_at FROM (SELECT q.provider AS provider,q.pool AS pool,q.window_key AS window_key,q.window_name AS window_name,o.source AS source,o.freshness AS freshness,q.used_fraction AS used_fraction,q.input_tokens AS input_tokens,q.cached_input_tokens AS cached_input_tokens,q.output_tokens AS output_tokens,q.reasoning_tokens AS reasoning_tokens,q.reset_at AS reset_at,o.observed_at AS observed_at,ROW_NUMBER() OVER (PARTITION BY q.provider,q.pool,q.window_key ORDER BY CASE o.freshness WHEN 'fresh' THEN 1 ELSE 0 END DESC,CASE o.source WHEN 'claude-api' THEN 4 WHEN 'codex-api' THEN 4 WHEN 'agy-api' THEN 4 WHEN 'structured-event' THEN 4 WHEN 'authenticated-cli' THEN 3 WHEN 'collect-all' THEN 3 WHEN 'registry' THEN 3 WHEN 'statusline' THEN 2 WHEN 'agy-meter' THEN 1 WHEN 'proxy' THEN 1 WHEN 'estimated' THEN 1 ELSE 0 END DESC,o.observed_at DESC,o.id DESC) AS best FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id WHERE q.provider=?) WHERE best=1 ORDER BY provider,pool,window_key`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Window
	for rows.Next() {
		var w Window
		var reset, observed sql.NullString
		var input, cached, output, reasoning sql.NullInt64
		if err := rows.Scan(&w.Provider, &w.Pool, &w.Key, &w.Name, &w.Source, &w.Freshness, &w.UsedFraction, &input, &cached, &output, &reasoning, &reset, &observed); err != nil {
			return nil, err
		}
		if input.Valid {
			v := input.Int64
			w.InputTokens = &v
		}
		if cached.Valid {
			v := cached.Int64
			w.CachedInputTokens = &v
		}
		if output.Valid {
			v := output.Int64
			w.OutputTokens = &v
		}
		if reasoning.Valid {
			v := reasoning.Int64
			w.ReasoningTokens = &v
		}
		var err error
		if w.ObservedAt, err = time.Parse(time.RFC3339Nano, observed.String); err != nil {
			return nil, err
		}
		if reset.Valid {
			t, e := time.Parse(time.RFC3339Nano, reset.String)
			if e != nil {
				return nil, e
			}
			w.ResetAt = &t
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// EnsureSchema creates only the additive store tables and indexes. Telemetry
// Open calls this on every initialization so existing database readers see a
// consistent schema before compact usage starts.
func EnsureSchema(ctx context.Context, exec func(context.Context, string) error) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS usage_observations (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, source TEXT NOT NULL, observed_at TEXT NOT NULL, freshness TEXT NOT NULL, payload_version TEXT NOT NULL DEFAULT '', UNIQUE(provider,source,observed_at))`,
		`CREATE TABLE IF NOT EXISTS quota_windows (id INTEGER PRIMARY KEY AUTOINCREMENT, observation_id INTEGER NOT NULL REFERENCES usage_observations(id) ON DELETE CASCADE, provider TEXT NOT NULL, pool TEXT NOT NULL DEFAULT '', window_key TEXT NOT NULL, window_name TEXT NOT NULL DEFAULT '', used_fraction REAL NOT NULL, input_tokens INTEGER, cached_input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER, reset_at TEXT, UNIQUE(observation_id,pool,window_key))`,
		`CREATE INDEX IF NOT EXISTS idx_usage_observations_provider_time ON usage_observations(provider,observed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_quota_windows_provider_pool_window ON quota_windows(provider,pool,window_key)`,
		`CREATE TABLE IF NOT EXISTS usage_load_observations (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, observed_at TEXT NOT NULL, payload_json TEXT NOT NULL, UNIQUE(provider,observed_at))`,
		`CREATE INDEX IF NOT EXISTS idx_usage_load_observations_provider_time ON usage_load_observations(provider,observed_at)`,
	} {
		if err := exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
