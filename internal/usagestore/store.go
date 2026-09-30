// Package usagestore persists normalized provider quota observations.
package usagestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// TurnQuotaBoundary preserves one quota capture at a turn boundary. Windows
// retain their individual source, timestamp, and normalized pool/window key.
type TurnQuotaBoundary struct {
	SessionID  string
	Turn       int
	Boundary   string
	Provider   string
	CapturedAt time.Time
	Source     string
	Freshness  string
	CacheAgeMS int64
	HasCache   bool
	Error      string
	ImportKey  string
	Windows    []Window
}

// TurnTokenUsage stores one token counter snapshot for a session turn.
type TurnTokenUsage struct {
	SessionID, Provider, CounterKind string
	Turn                             int
	InputTokens, CachedInputTokens   *int64
	OutputTokens, ReasoningTokens    *int64
	InputQuality, CachedInputQuality string
	OutputQuality, ReasoningQuality  string
	ObservedAt                       time.Time
}

// TurnQuotaDelta is a same-provider, same-pool, same-window comparison.
type TurnQuotaDelta struct {
	SessionID, Provider, Pool, WindowKey string
	Turn                                 int
	BeforeUsed, AfterUsed, Delta         float64
	BeforeAt, AfterAt                    time.Time
	BeforeSource, AfterSource            string
	BeforeReset, AfterReset              *time.Time
	BeforeCacheAgeMS, AfterCacheAgeMS    int64
	BeforeHasCache, AfterHasCache        bool
	BeforeError, AfterError              string
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

// MigrateStableWindowKeys canonicalizes provider window aliases while
// retaining each previous key. Rows that collapse onto the same observation,
// pool, and canonical key are copied to an archive table before the duplicate
// is removed from reader-facing tables.
func (s *Store) MigrateStableWindowKeys(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var completed int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name='normalized-window-keys-v2'`).Scan(&completed); err != nil {
		return err
	}
	if completed > 0 {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS usage_window_key_migration_archive (id INTEGER PRIMARY KEY, observation_id INTEGER NOT NULL, provider TEXT NOT NULL, pool TEXT NOT NULL, original_window_key TEXT NOT NULL, window_name TEXT NOT NULL, used_fraction REAL NOT NULL, input_tokens INTEGER, cached_input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER, reset_at TEXT)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE usage_observations SET freshness='stale' WHERE id IN (SELECT observation_id FROM quota_windows WHERE lower(window_name) LIKE '%(stale)%')`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,observation_id,provider,pool,window_key,window_name,used_fraction,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,reset_at FROM quota_windows ORDER BY id`)
	if err != nil {
		return err
	}
	type row struct {
		id, observationID                int64
		provider, pool, key, name        string
		used                             float64
		input, cached, output, reasoning sql.NullInt64
		reset                            sql.NullString
	}
	var records []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.observationID, &r.provider, &r.pool, &r.key, &r.name, &r.used, &r.input, &r.cached, &r.output, &r.reasoning, &r.reset); err != nil {
			rows.Close()
			return err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, r := range records {
		name := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(r.name, " (stale)", ""), " (STALE)", ""))
		label := r.name
		if strings.TrimSpace(label) == "" {
			label = r.key
		}
		key := NormalizeWindowKey(r.provider, r.pool, label)
		if key == r.key && name == r.name {
			continue
		}
		var duplicate int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM quota_windows WHERE observation_id=? AND pool=? AND window_key=? AND id<>?`, r.observationID, r.pool, key, r.id).Scan(&duplicate)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO usage_window_key_migration_archive(id,observation_id,provider,pool,original_window_key,window_name,used_fraction,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,reset_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, r.id, r.observationID, r.provider, r.pool, r.key, r.name, r.used, nullIntArg(r.input), nullIntArg(r.cached), nullIntArg(r.output), nullIntArg(r.reasoning), nullStringArg(r.reset)); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM quota_windows WHERE id=?`, r.id); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE quota_windows SET window_key=?,window_name=?,legacy_window_key=CASE WHEN window_key<>? AND legacy_window_key='' THEN window_key ELSE legacy_window_key END WHERE id=?`, key, name, key, r.id); err != nil {
			return err
		}
	}
	if err := migrateTurnDeltaWindowKeys(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO usage_store_migrations(name,completed_at) VALUES('normalized-window-keys-v2',?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

func migrateTurnDeltaWindowKeys(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS turn_quota_delta_key_migration_archive AS SELECT * FROM turn_quota_deltas WHERE 0`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,provider,pool,window_key FROM turn_quota_deltas ORDER BY id`)
	if err != nil {
		return err
	}
	type deltaKey struct {
		id                  int64
		provider, pool, key string
	}
	var records []deltaKey
	for rows.Next() {
		var row deltaKey
		if err := rows.Scan(&row.id, &row.provider, &row.pool, &row.key); err != nil {
			rows.Close()
			return err
		}
		records = append(records, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, row := range records {
		key := NormalizeWindowKey(row.provider, row.pool, row.key)
		if key == row.key {
			continue
		}
		var duplicate int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM turn_quota_deltas WHERE session_id=(SELECT session_id FROM turn_quota_deltas WHERE id=?) AND turn=(SELECT turn FROM turn_quota_deltas WHERE id=?) AND provider=? AND pool=? AND window_key=? AND id<>?`, row.id, row.id, row.provider, row.pool, key, row.id).Scan(&duplicate)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO turn_quota_delta_key_migration_archive SELECT * FROM turn_quota_deltas WHERE id=?`, row.id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM turn_quota_deltas WHERE id=?`, row.id); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE turn_quota_deltas SET window_key=?,legacy_window_key=? WHERE id=?`, key, row.key, row.id); err != nil {
			return err
		}
	}
	return nil
}

func nullIntArg(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullStringArg(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
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
		if w.Name == "" {
			w.Name = w.Key
		}
		w.Key = NormalizeWindowKey(w.Provider, w.Pool, w.Name)
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

// WriteTurnQuotaBoundary stores the event and its source-tagged quota windows.
// ImportKey makes JSONL imports safe to retry without suppressing live events.
func (s *Store) WriteTurnQuotaBoundary(ctx context.Context, boundary TurnQuotaBoundary) (int64, error) {
	if boundary.SessionID == "" || boundary.Turn < 1 || boundary.Provider == "" || (boundary.Boundary != "before" && boundary.Boundary != "after") {
		return 0, fmt.Errorf("invalid turn quota boundary")
	}
	if boundary.CapturedAt.IsZero() {
		boundary.CapturedAt = time.Now().UTC()
	}
	if boundary.Source == "" {
		boundary.Source = "turn-capture"
	}
	if boundary.Freshness == "" {
		boundary.Freshness = "unknown"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO turn_quota_boundaries(session_id,turn,boundary,provider,captured_at,source,freshness,cache_age_ms,has_cache,error,import_key) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, boundary.SessionID, boundary.Turn, boundary.Boundary, boundary.Provider, boundary.CapturedAt.UTC().Format(time.RFC3339Nano), boundary.Source, boundary.Freshness, boundary.CacheAgeMS, boundary.HasCache, boundary.Error, boundary.ImportKey)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if boundary.ImportKey != "" {
		if err := tx.QueryRowContext(ctx, `SELECT id FROM turn_quota_boundaries WHERE import_key=?`, boundary.ImportKey).Scan(&id); err != nil {
			return 0, err
		}
	}
	if len(boundary.Windows) == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO usage_observations(provider,source,observed_at,freshness,turn_boundary_id) VALUES(?,?,?,?,?)`, boundary.Provider, boundary.Source, boundary.CapturedAt.UTC().Format(time.RFC3339Nano), boundary.Freshness, id); err != nil {
			return 0, err
		}
	}
	for _, w := range boundary.Windows {
		if w.Provider == "" {
			w.Provider = boundary.Provider
		}
		if w.Name == "" {
			w.Name = w.Key
		}
		w.Key = NormalizeWindowKey(w.Provider, w.Pool, w.Name)
		if w.ObservedAt.IsZero() {
			w.ObservedAt = boundary.CapturedAt
		}
		if w.Source == "" {
			w.Source = boundary.Source
		}
		if w.Freshness == "" {
			w.Freshness = boundary.Freshness
		}
		var reset any
		if w.ResetAt != nil {
			reset = w.ResetAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_observations(provider,source,observed_at,freshness,turn_boundary_id) VALUES(?,?,?,?,?) ON CONFLICT(provider,source,observed_at) DO UPDATE SET turn_boundary_id=COALESCE(usage_observations.turn_boundary_id,excluded.turn_boundary_id)`, w.Provider, w.Source, w.ObservedAt.UTC().Format(time.RFC3339Nano), w.Freshness, id); err != nil {
			return 0, err
		}
		var observationID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM usage_observations WHERE provider=? AND source=? AND observed_at=?`, w.Provider, w.Source, w.ObservedAt.UTC().Format(time.RFC3339Nano)).Scan(&observationID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO quota_windows(observation_id,provider,pool,window_key,window_name,used_fraction,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,reset_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(observation_id,pool,window_key) DO UPDATE SET window_name=excluded.window_name,used_fraction=excluded.used_fraction,reset_at=excluded.reset_at`, observationID, w.Provider, w.Pool, w.Key, w.Name, w.UsedFraction, w.InputTokens, w.CachedInputTokens, w.OutputTokens, w.ReasoningTokens, reset); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// PairTurnQuotaDeltas records valid before/after pairs for matching windows.
// Pairs with reversed timestamps, changed reset markers, or decreased usage
// are omitted because they crossed or may have crossed a provider reset.
func (s *Store) PairTurnQuotaDeltas(ctx context.Context, sessionID string, turn int, provider string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO turn_quota_deltas(session_id,turn,provider,pool,window_key,before_boundary_id,after_boundary_id,before_observation_id,after_observation_id,before_used_fraction,after_used_fraction,delta_used_fraction,before_source,after_source,before_observed_at,after_observed_at,before_reset_at,after_reset_at,before_cache_age_ms,after_cache_age_ms,before_has_cache,after_has_cache,before_error,after_error)
		SELECT b.session_id,b.turn,b.provider,qb.pool,qb.window_key,b.id,a.id,ob.id,oa.id,qb.used_fraction,qa.used_fraction,qa.used_fraction-qb.used_fraction,ob.source,oa.source,ob.observed_at,oa.observed_at,qb.reset_at,qa.reset_at,b.cache_age_ms,a.cache_age_ms,b.has_cache,a.has_cache,b.error,a.error
		FROM turn_quota_boundaries b JOIN turn_quota_boundaries a ON a.session_id=b.session_id AND a.turn=b.turn AND a.provider=b.provider AND a.boundary='after'
		JOIN quota_windows qb ON qb.observation_id IN (SELECT id FROM usage_observations WHERE turn_boundary_id=b.id)
		JOIN usage_observations ob ON ob.id=qb.observation_id
		JOIN quota_windows qa ON qa.provider=qb.provider AND qa.pool=qb.pool AND qa.window_key=qb.window_key AND qa.observation_id IN (SELECT id FROM usage_observations WHERE turn_boundary_id=a.id)
		JOIN usage_observations oa ON oa.id=qa.observation_id
		WHERE b.session_id=? AND b.turn=? AND b.provider=? AND b.boundary='before' AND b.id=(SELECT MAX(id) FROM turn_quota_boundaries WHERE session_id=b.session_id AND turn=b.turn AND provider=b.provider AND boundary='before') AND a.id=(SELECT MAX(id) FROM turn_quota_boundaries WHERE session_id=a.session_id AND turn=a.turn AND provider=a.provider AND boundary='after') AND b.captured_at<a.captured_at AND COALESCE(qb.reset_at,'')=COALESCE(qa.reset_at,'') AND qa.used_fraction>=qb.used_fraction
		ON CONFLICT(session_id,turn,provider,pool,window_key) DO UPDATE SET before_boundary_id=excluded.before_boundary_id,after_boundary_id=excluded.after_boundary_id,before_observation_id=excluded.before_observation_id,after_observation_id=excluded.after_observation_id,before_used_fraction=excluded.before_used_fraction,after_used_fraction=excluded.after_used_fraction,delta_used_fraction=excluded.delta_used_fraction,before_source=excluded.before_source,after_source=excluded.after_source,before_observed_at=excluded.before_observed_at,after_observed_at=excluded.after_observed_at,before_reset_at=excluded.before_reset_at,after_reset_at=excluded.after_reset_at,before_cache_age_ms=excluded.before_cache_age_ms,after_cache_age_ms=excluded.after_cache_age_ms,before_has_cache=excluded.before_has_cache,after_has_cache=excluded.after_has_cache,before_error=excluded.before_error,after_error=excluded.after_error`, sessionID, turn, provider)
	return err
}

// WriteTurnTokenUsage upserts one cumulative or per-turn counter snapshot.
func (s *Store) WriteTurnTokenUsage(ctx context.Context, usage TurnTokenUsage) error {
	if usage.SessionID == "" || usage.Turn < 1 || usage.Provider == "" || (usage.CounterKind != "cumulative" && usage.CounterKind != "delta") {
		return fmt.Errorf("invalid turn token usage")
	}
	if usage.ObservedAt.IsZero() {
		usage.ObservedAt = time.Now().UTC()
	}
	for _, quality := range []string{usage.InputQuality, usage.CachedInputQuality, usage.OutputQuality, usage.ReasoningQuality} {
		if quality != "measured" && quality != "fitted" && quality != "shared" && quality != "unknown" {
			return fmt.Errorf("invalid token quality %q", quality)
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO turn_token_usage(session_id,turn,provider,counter_kind,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,input_quality,cached_input_quality,output_quality,reasoning_quality,observed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(session_id,turn,counter_kind) DO UPDATE SET provider=excluded.provider,input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,output_tokens=excluded.output_tokens,reasoning_tokens=excluded.reasoning_tokens,input_quality=excluded.input_quality,cached_input_quality=excluded.cached_input_quality,output_quality=excluded.output_quality,reasoning_quality=excluded.reasoning_quality,observed_at=excluded.observed_at`, usage.SessionID, usage.Turn, usage.Provider, usage.CounterKind, usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ReasoningTokens, usage.InputQuality, usage.CachedInputQuality, usage.OutputQuality, usage.ReasoningQuality, usage.ObservedAt.UTC().Format(time.RFC3339Nano))
	return err
}

// TurnQuotaDeltas returns the ordered paired deltas for one session turn.
func (s *Store) TurnQuotaDeltas(ctx context.Context, sessionID string, turn int) ([]TurnQuotaDelta, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id,turn,provider,pool,window_key,before_used_fraction,after_used_fraction,delta_used_fraction,before_observed_at,after_observed_at,before_source,after_source,before_reset_at,after_reset_at,before_cache_age_ms,after_cache_age_ms,before_has_cache,after_has_cache,before_error,after_error FROM turn_quota_deltas WHERE session_id=? AND turn=? ORDER BY provider,pool,window_key`, sessionID, turn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TurnQuotaDelta
	for rows.Next() {
		var d TurnQuotaDelta
		var beforeAt, afterAt string
		var beforeReset, afterReset sql.NullString
		if err := rows.Scan(&d.SessionID, &d.Turn, &d.Provider, &d.Pool, &d.WindowKey, &d.BeforeUsed, &d.AfterUsed, &d.Delta, &beforeAt, &afterAt, &d.BeforeSource, &d.AfterSource, &beforeReset, &afterReset, &d.BeforeCacheAgeMS, &d.AfterCacheAgeMS, &d.BeforeHasCache, &d.AfterHasCache, &d.BeforeError, &d.AfterError); err != nil {
			return nil, err
		}
		if d.BeforeAt, err = time.Parse(time.RFC3339Nano, beforeAt); err != nil {
			return nil, err
		}
		if d.AfterAt, err = time.Parse(time.RFC3339Nano, afterAt); err != nil {
			return nil, err
		}
		if beforeReset.Valid {
			t, parseErr := time.Parse(time.RFC3339Nano, beforeReset.String)
			if parseErr != nil {
				return nil, parseErr
			}
			d.BeforeReset = &t
		}
		if afterReset.Valid {
			t, parseErr := time.Parse(time.RFC3339Nano, afterReset.String)
			if parseErr != nil {
				return nil, parseErr
			}
			d.AfterReset = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// TurnQuotaDeltasForSession returns all valid quota pairs for one session.
func (s *Store) TurnQuotaDeltasForSession(ctx context.Context, sessionID string) ([]TurnQuotaDelta, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id,turn,provider,pool,window_key,before_used_fraction,after_used_fraction,delta_used_fraction,before_observed_at,after_observed_at,before_source,after_source,before_reset_at,after_reset_at,before_cache_age_ms,after_cache_age_ms,before_has_cache,after_has_cache,before_error,after_error FROM turn_quota_deltas WHERE session_id=? ORDER BY turn,provider,pool,window_key`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TurnQuotaDelta
	for rows.Next() {
		var d TurnQuotaDelta
		var beforeAt, afterAt string
		var beforeReset, afterReset sql.NullString
		if err := rows.Scan(&d.SessionID, &d.Turn, &d.Provider, &d.Pool, &d.WindowKey, &d.BeforeUsed, &d.AfterUsed, &d.Delta, &beforeAt, &afterAt, &d.BeforeSource, &d.AfterSource, &beforeReset, &afterReset, &d.BeforeCacheAgeMS, &d.AfterCacheAgeMS, &d.BeforeHasCache, &d.AfterHasCache, &d.BeforeError, &d.AfterError); err != nil {
			return nil, err
		}
		if d.BeforeAt, err = time.Parse(time.RFC3339Nano, beforeAt); err != nil {
			return nil, err
		}
		if d.AfterAt, err = time.Parse(time.RFC3339Nano, afterAt); err != nil {
			return nil, err
		}
		if beforeReset.Valid {
			t, parseErr := time.Parse(time.RFC3339Nano, beforeReset.String)
			if parseErr != nil {
				return nil, parseErr
			}
			d.BeforeReset = &t
		}
		if afterReset.Valid {
			t, parseErr := time.Parse(time.RFC3339Nano, afterReset.String)
			if parseErr != nil {
				return nil, parseErr
			}
			d.AfterReset = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// TurnTokenUsages returns the stored delta and cumulative rows for a session.
func (s *Store) TurnTokenUsages(ctx context.Context, sessionID string) ([]TurnTokenUsage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id,turn,provider,counter_kind,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,input_quality,cached_input_quality,output_quality,reasoning_quality,observed_at FROM turn_token_usage WHERE session_id=? ORDER BY turn,counter_kind`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TurnTokenUsage
	for rows.Next() {
		var token TurnTokenUsage
		var input, cached, output, reasoning sql.NullInt64
		var observed string
		if err := rows.Scan(&token.SessionID, &token.Turn, &token.Provider, &token.CounterKind, &input, &cached, &output, &reasoning, &token.InputQuality, &token.CachedInputQuality, &token.OutputQuality, &token.ReasoningQuality, &observed); err != nil {
			return nil, err
		}
		if input.Valid {
			v := input.Int64
			token.InputTokens = &v
		}
		if cached.Valid {
			v := cached.Int64
			token.CachedInputTokens = &v
		}
		if output.Valid {
			v := output.Int64
			token.OutputTokens = &v
		}
		if reasoning.Valid {
			v := reasoning.Int64
			token.ReasoningTokens = &v
		}
		if token.ObservedAt, err = time.Parse(time.RFC3339Nano, observed); err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

// RebuildTurnTokenCumulative derives cumulative compatibility rows from
// imported delta rows while keeping unknown dimensions unknown.
func (s *Store) RebuildTurnTokenCumulative(ctx context.Context, sessionID string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT turn,provider,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,input_quality,cached_input_quality,output_quality,reasoning_quality,observed_at FROM turn_token_usage WHERE session_id=? AND counter_kind='delta' ORDER BY turn`, sessionID)
	if err != nil {
		return err
	}
	type deltaRow struct {
		turn                             int
		provider                         string
		input, cached, output, reasoning sql.NullInt64
		iq, cq, oq, rq, observed         string
	}
	var deltas []deltaRow
	for rows.Next() {
		var row deltaRow
		if err := rows.Scan(&row.turn, &row.provider, &row.input, &row.cached, &row.output, &row.reasoning, &row.iq, &row.cq, &row.oq, &row.rq, &row.observed); err != nil {
			rows.Close()
			return err
		}
		deltas = append(deltas, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	type total struct {
		value   int64
		known   bool
		quality string
	}
	inputs, cacheds, outputs, reasonings := total{known: true}, total{known: true}, total{known: true}, total{known: true}
	for _, row := range deltas {
		accumulate := func(total *total, value sql.NullInt64, quality string) {
			if !value.Valid || quality == "unknown" {
				total.known = false
				total.quality = "unknown"
				return
			}
			total.value += value.Int64
			if total.quality == "" {
				total.quality = quality
			} else if total.quality != quality {
				total.quality = "shared"
			}
		}
		accumulate(&inputs, row.input, row.iq)
		accumulate(&cacheds, row.cached, row.cq)
		accumulate(&outputs, row.output, row.oq)
		accumulate(&reasonings, row.reasoning, row.rq)
		quality := func(total total) (any, string) {
			if !total.known {
				return nil, "unknown"
			}
			v := total.value
			return &v, total.quality
		}
		iv, iqual := quality(inputs)
		cv, cqual := quality(cacheds)
		ov, oqual := quality(outputs)
		rv, rqual := quality(reasonings)
		if _, err := s.db.ExecContext(ctx, `INSERT INTO turn_token_usage(session_id,turn,provider,counter_kind,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,input_quality,cached_input_quality,output_quality,reasoning_quality,observed_at) VALUES(?,?,?,'cumulative',?,?,?,?,?,?,?,?,?) ON CONFLICT(session_id,turn,counter_kind) DO UPDATE SET provider=excluded.provider,input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,output_tokens=excluded.output_tokens,reasoning_tokens=excluded.reasoning_tokens,input_quality=excluded.input_quality,cached_input_quality=excluded.cached_input_quality,output_quality=excluded.output_quality,reasoning_quality=excluded.reasoning_quality,observed_at=excluded.observed_at`, sessionID, row.turn, row.provider, iv, cv, ov, rv, iqual, cqual, oqual, rqual, row.observed); err != nil {
			return err
		}
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

// QuotaHistory returns every normalized quota observation with provenance.
func (s *Store) QuotaHistory(ctx context.Context) ([]Window, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT q.provider,q.pool,q.window_key,q.window_name,o.source,o.freshness,q.used_fraction,q.reset_at,o.observed_at FROM quota_windows q JOIN usage_observations o ON o.id=q.observation_id ORDER BY o.observed_at,q.provider,q.pool,q.window_key,o.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Window
	for rows.Next() {
		var window Window
		var reset sql.NullString
		var observed string
		if err := rows.Scan(&window.Provider, &window.Pool, &window.Key, &window.Name, &window.Source, &window.Freshness, &window.UsedFraction, &reset, &observed); err != nil {
			return nil, err
		}
		if window.ObservedAt, err = time.Parse(time.RFC3339Nano, observed); err != nil {
			return nil, err
		}
		if reset.Valid {
			t, parseErr := time.Parse(time.RFC3339Nano, reset.String)
			if parseErr != nil {
				return nil, parseErr
			}
			window.ResetAt = &t
		}
		out = append(out, window)
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
	bestByWindow := make(map[string]Window)
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
		w.Key = NormalizeWindowKey(w.Provider, w.Pool, w.Name)
		group := w.Provider + "\x00" + w.Pool + "\x00" + w.Key
		if previous, ok := bestByWindow[group]; !ok || windowRanksHigher(w, previous) {
			bestByWindow[group] = w
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Window, 0, len(bestByWindow))
	for _, w := range bestByWindow {
		out = append(out, w)
	}
	return out, nil
}

// NormalizeWindowKey maps display labels to stable machine-readable keys.
func NormalizeWindowKey(provider, pool, name string) string {
	name = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(name, " (stale)", "")))
	name = strings.TrimSuffix(name, "_limit_remaining")
	name = strings.TrimSuffix(name, " limit remaining")
	name = strings.TrimSuffix(name, "-limit-remaining")
	compact := strings.NewReplacer("_", "", "-", "", " ", "").Replace(name)
	switch {
	case strings.Contains(name, "week"), strings.Contains(name, "7-day"), strings.Contains(name, "7 day"), strings.Contains(compact, "7d"), strings.Contains(compact, "sevenday"):
		return "weekly"
	case strings.Contains(name, "5-hour"), strings.Contains(name, "5 hour"), strings.Contains(compact, "fivehour"), strings.Contains(compact, "5h"), strings.Contains(name, "session"):
		return "five_hour"
	case strings.Contains(name, "spend"):
		return "spend_limit"
	}
	var key strings.Builder
	separator := false
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && key.Len() > 0 {
				key.WriteByte('_')
			}
			key.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	if key.Len() == 0 {
		if pool != "" {
			return "quota"
		}
		return strings.ToLower(provider) + "_quota"
	}
	return key.String()
}

func windowRanksHigher(candidate, previous Window) bool {
	if (candidate.Freshness == "fresh") != (previous.Freshness == "fresh") {
		return candidate.Freshness == "fresh"
	}
	if sourcePriority(candidate.Source) != sourcePriority(previous.Source) {
		return sourcePriority(candidate.Source) > sourcePriority(previous.Source)
	}
	return candidate.ObservedAt.After(previous.ObservedAt)
}

func sourcePriority(source string) int {
	switch source {
	case "claude-api", "codex-api", "agy-api", "structured-event":
		return 4
	case "authenticated-cli", "collect-all", "registry":
		return 3
	case "statusline":
		return 2
	case "agy-meter", "proxy", "estimated":
		return 1
	default:
		return 0
	}
}

// EnsureSchema creates only the additive store tables and indexes. Telemetry
// Open calls this on every initialization so existing database readers see a
// consistent schema before compact usage starts.
func EnsureSchema(ctx context.Context, exec func(context.Context, string) error) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS usage_observations (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, source TEXT NOT NULL, observed_at TEXT NOT NULL, freshness TEXT NOT NULL, payload_version TEXT NOT NULL DEFAULT '', turn_boundary_id INTEGER REFERENCES turn_quota_boundaries(id), UNIQUE(provider,source,observed_at))`,
		`CREATE TABLE IF NOT EXISTS quota_windows (id INTEGER PRIMARY KEY AUTOINCREMENT, observation_id INTEGER NOT NULL REFERENCES usage_observations(id) ON DELETE CASCADE, provider TEXT NOT NULL, pool TEXT NOT NULL DEFAULT '', window_key TEXT NOT NULL, window_name TEXT NOT NULL DEFAULT '', used_fraction REAL NOT NULL, input_tokens INTEGER, cached_input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER, reset_at TEXT, UNIQUE(observation_id,pool,window_key))`,
		`CREATE TABLE IF NOT EXISTS turn_quota_boundaries (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, turn INTEGER NOT NULL, boundary TEXT NOT NULL, provider TEXT NOT NULL, captured_at TEXT NOT NULL, source TEXT NOT NULL, freshness TEXT NOT NULL, cache_age_ms INTEGER NOT NULL DEFAULT 0, has_cache INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '', import_key TEXT NOT NULL DEFAULT '')`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_turn_quota_boundaries_import_key ON turn_quota_boundaries(import_key) WHERE import_key<>''`,
		`CREATE INDEX IF NOT EXISTS idx_turn_quota_boundaries_session_turn ON turn_quota_boundaries(session_id,turn,boundary)`,
		`CREATE TABLE IF NOT EXISTS turn_quota_deltas (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, turn INTEGER NOT NULL, provider TEXT NOT NULL, pool TEXT NOT NULL DEFAULT '', window_key TEXT NOT NULL, before_boundary_id INTEGER NOT NULL REFERENCES turn_quota_boundaries(id), after_boundary_id INTEGER NOT NULL REFERENCES turn_quota_boundaries(id), before_observation_id INTEGER NOT NULL REFERENCES usage_observations(id), after_observation_id INTEGER NOT NULL REFERENCES usage_observations(id), before_used_fraction REAL NOT NULL, after_used_fraction REAL NOT NULL, delta_used_fraction REAL NOT NULL, before_source TEXT NOT NULL, after_source TEXT NOT NULL, before_observed_at TEXT NOT NULL, after_observed_at TEXT NOT NULL, before_reset_at TEXT, after_reset_at TEXT, before_cache_age_ms INTEGER NOT NULL DEFAULT 0, after_cache_age_ms INTEGER NOT NULL DEFAULT 0, before_has_cache INTEGER NOT NULL DEFAULT 0, after_has_cache INTEGER NOT NULL DEFAULT 0, before_error TEXT NOT NULL DEFAULT '', after_error TEXT NOT NULL DEFAULT '', UNIQUE(session_id,turn,provider,pool,window_key))`,
		`CREATE INDEX IF NOT EXISTS idx_turn_quota_deltas_session_turn ON turn_quota_deltas(session_id,turn)`,
		`CREATE TABLE IF NOT EXISTS turn_token_usage (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, turn INTEGER NOT NULL, provider TEXT NOT NULL, counter_kind TEXT NOT NULL, input_tokens INTEGER, cached_input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER, input_quality TEXT NOT NULL DEFAULT 'unknown', cached_input_quality TEXT NOT NULL DEFAULT 'unknown', output_quality TEXT NOT NULL DEFAULT 'unknown', reasoning_quality TEXT NOT NULL DEFAULT 'unknown', observed_at TEXT NOT NULL, UNIQUE(session_id,turn,counter_kind))`,
		`CREATE INDEX IF NOT EXISTS idx_turn_token_usage_session_turn ON turn_token_usage(session_id,turn)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_observations_provider_time ON usage_observations(provider,observed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_quota_windows_provider_pool_window ON quota_windows(provider,pool,window_key)`,
		`CREATE TABLE IF NOT EXISTS usage_load_observations (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, observed_at TEXT NOT NULL, payload_json TEXT NOT NULL, UNIQUE(provider,observed_at))`,
		`CREATE INDEX IF NOT EXISTS idx_usage_load_observations_provider_time ON usage_load_observations(provider,observed_at)`,
	} {
		if err := exec(ctx, statement); err != nil {
			return err
		}
	}
	// quota_windows predated the typed token columns. CREATE TABLE IF NOT
	// EXISTS does not upgrade existing installations, so add these columns
	// individually. SQLite reports duplicate-column when this schema is
	// already current; tolerate only that idempotent case.
	for _, column := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_tokens"} {
		if err := exec(ctx, `ALTER TABLE quota_windows ADD COLUMN `+column+` INTEGER`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	if err := exec(ctx, `ALTER TABLE quota_windows ADD COLUMN legacy_window_key TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return err
	}
	if err := exec(ctx, `ALTER TABLE turn_quota_deltas ADD COLUMN legacy_window_key TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return err
	}
	if err := exec(ctx, `ALTER TABLE usage_observations ADD COLUMN turn_boundary_id INTEGER REFERENCES turn_quota_boundaries(id)`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return err
	}
	for _, column := range []struct{ name, definition string }{{"before_cache_age_ms", "INTEGER NOT NULL DEFAULT 0"}, {"after_cache_age_ms", "INTEGER NOT NULL DEFAULT 0"}, {"before_has_cache", "INTEGER NOT NULL DEFAULT 0"}, {"after_has_cache", "INTEGER NOT NULL DEFAULT 0"}, {"before_error", "TEXT NOT NULL DEFAULT ''"}, {"after_error", "TEXT NOT NULL DEFAULT ''"}} {
		if err := exec(ctx, `ALTER TABLE turn_quota_deltas ADD COLUMN `+column.name+` `+column.definition); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	return nil
}
