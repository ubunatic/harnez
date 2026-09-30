package usage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

// TurnQuotaImportResult reports source event rows and their durable imported
// boundary rows. Invalid JSONL lines are counted separately for parity checks.
type TurnQuotaImportResult struct {
	SourceEvents int
	ImportedRows int
	InvalidRows  int
	AlreadyDone  bool
}

type legacyTurnQuotaEvent struct {
	SessionID string           `json:"session_id"`
	Turn      int              `json:"turn"`
	Boundary  string           `json:"boundary"`
	Provider  string           `json:"provider"`
	Reading   TurnQuotaReading `json:"reading"`
	Tokens    *struct {
		NewInputTokens    int `json:"new_input_tokens"`
		CachedInputTokens int `json:"cached_input_tokens"`
		OutputTokens      int `json:"output_tokens"`
		ReasoningTokens   int `json:"reasoning_tokens,omitempty"`
	} `json:"tokens,omitempty"`
}

func openTurnUsageStore(ctx context.Context, dbPath string) (*usagestore.Store, error) {
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return nil, err
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return nil, err
	}
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

// PersistTurnQuotaBoundary writes one turn boundary and pairs its after windows
// with matching non-reset before windows.
func PersistTurnQuotaBoundary(ctx context.Context, dbPath string, boundary usagestore.TurnQuotaBoundary) error {
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err := store.WriteTurnQuotaBoundary(ctx, boundary); err != nil {
		return err
	}
	if boundary.Boundary == "after" {
		return store.PairTurnQuotaDeltas(ctx, boundary.SessionID, boundary.Turn, boundary.Provider)
	}
	return nil
}

// TurnQuotaBoundaryFromReading normalizes one provider capture for durable
// turn storage while keeping legacy rounded windows as an import fallback.
func TurnQuotaBoundaryFromReading(sessionID, provider string, turn int, boundary string, reading TurnQuotaReading) usagestore.TurnQuotaBoundary {
	return legacyBoundary(legacyTurnQuotaEvent{
		SessionID: sessionID, Turn: turn, Boundary: boundary, Provider: provider, Reading: reading,
	}, "")
}

// PersistTurnTokenUsage writes one cumulative or delta snapshot.
func PersistTurnTokenUsage(ctx context.Context, dbPath string, tokens usagestore.TurnTokenUsage) error {
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.WriteTurnTokenUsage(ctx, tokens)
}

// TurnQuotaDeltasForSession reads paired quota records through the shared
// store API.
func TurnQuotaDeltasForSession(ctx context.Context, dbPath, sessionID string) ([]usagestore.TurnQuotaDelta, error) {
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.TurnQuotaDeltasForSession(ctx, sessionID)
}

// TurnTokenUsagesForSession reads cumulative and per-turn token rows through
// the shared store API.
func TurnTokenUsagesForSession(ctx context.Context, dbPath, sessionID string) ([]usagestore.TurnTokenUsage, error) {
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.TurnTokenUsages(ctx, sessionID)
}

// ImportTurnQuotaJSONL imports the legacy agent turn spool once per source
// path. Import keys include the line bytes and ordinal so interrupted imports
// can safely resume without duplicating event rows.
func ImportTurnQuotaJSONL(ctx context.Context, dbPath, path string) (TurnQuotaImportResult, error) {
	result := TurnQuotaImportResult{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer f.Close()
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return result, err
	}
	defer store.Close()
	if err := store.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return result, err
	}
	pathHash := sha256.Sum256([]byte(filepath.Clean(path)))
	prefix := "turn-jsonl:" + hex.EncodeToString(pathHash[:8]) + ":"
	marker := "turn-jsonl-import-v1:" + hex.EncodeToString(pathHash[:8])
	var done int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name=?`, marker).Scan(&done); err != nil {
		return result, err
	}
	if done > 0 {
		result.AlreadyDone = true
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNumber := 0
	tokenSessions := map[string]bool{}
	for scanner.Scan() {
		lineNumber++
		line := append([]byte(nil), scanner.Bytes()...)
		var event legacyTurnQuotaEvent
		if json.Unmarshal(line, &event) != nil || event.SessionID == "" || event.Turn < 1 || event.Provider == "" || (event.Boundary != "before" && event.Boundary != "after") {
			result.InvalidRows++
			continue
		}
		result.SourceEvents++
		lineHash := sha256.Sum256(line)
		importKey := prefix + fmt.Sprintf("%d:", lineNumber) + hex.EncodeToString(lineHash[:])
		boundary := legacyBoundary(event, importKey)
		if _, err := store.WriteTurnQuotaBoundary(ctx, boundary); err != nil {
			return result, fmt.Errorf("import turn quota line %d: %w", lineNumber, err)
		}
		if event.Boundary == "after" {
			if err := store.PairTurnQuotaDeltas(ctx, event.SessionID, event.Turn, event.Provider); err != nil {
				return result, err
			}
		}
		if event.Tokens != nil {
			if err := importLegacyTurnTokens(ctx, store, event); err != nil {
				return result, fmt.Errorf("import turn token line %d: %w", lineNumber, err)
			}
			tokenSessions[event.SessionID] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	for sessionID := range tokenSessions {
		if err := store.RebuildTurnTokenCumulative(ctx, sessionID); err != nil {
			return result, err
		}
	}
	if !result.AlreadyDone {
		if err := store.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES(?,?)`, marker, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return result, err
		}
	}
	if err := store.QueryRow(ctx, `SELECT count(*) FROM turn_quota_boundaries WHERE substr(import_key,1,?)=?`, len(prefix), prefix).Scan(&result.ImportedRows); err != nil {
		return result, err
	}
	return result, nil
}

func legacyBoundary(event legacyTurnQuotaEvent, importKey string) usagestore.TurnQuotaBoundary {
	reading := event.Reading
	at := reading.CapturedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	source := reading.Source
	if source == "" {
		source = "turn-capture"
	}
	freshness := "fresh"
	if reading.Error != "" || !reading.HasCache || reading.CacheAgeMS > int64(DefaultCacheStaleness/time.Millisecond) {
		freshness = "stale"
	}
	windows := append([]usagestore.Window(nil), reading.StoreWindows...)
	if len(windows) == 0 {
		for _, old := range reading.Windows {
			windowAt := old.Timestamp
			if windowAt.IsZero() {
				windowAt = at
			}
			pool := old.Group
			windows = append(windows, usagestore.Window{
				Provider: event.Provider, Pool: pool,
				Key: usagestore.NormalizeWindowKey(event.Provider, "", old.Window), Name: old.Window,
				Source: source, Freshness: freshness, UsedFraction: float64(old.UsedPercent) / 100,
				ResetAt: old.ResetAt, ObservedAt: windowAt,
			})
		}
	}
	for i := range windows {
		if windows[i].Provider == "" {
			windows[i].Provider = event.Provider
		}
		if windows[i].Key == "" {
			windows[i].Key = usagestore.NormalizeWindowKey(event.Provider, "", windows[i].Name)
		}
	}
	return usagestore.TurnQuotaBoundary{
		SessionID: event.SessionID, Turn: event.Turn, Boundary: event.Boundary, Provider: event.Provider,
		CapturedAt: at, Source: source, Freshness: freshness, CacheAgeMS: reading.CacheAgeMS,
		HasCache: reading.HasCache, Error: reading.Error, ImportKey: importKey, Windows: windows,
	}
}

func importLegacyTurnTokens(ctx context.Context, store *usagestore.Store, event legacyTurnQuotaEvent) error {
	input, cached, output := int64(event.Tokens.NewInputTokens), int64(event.Tokens.CachedInputTokens), int64(event.Tokens.OutputTokens)
	reasoning := int64(event.Tokens.ReasoningTokens)
	observedAt := event.Reading.CapturedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	tokens := usagestore.TurnTokenUsage{
		SessionID: event.SessionID, Turn: event.Turn, Provider: event.Provider, CounterKind: "delta",
		InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output,
		InputQuality: "measured", CachedInputQuality: "measured", OutputQuality: "measured", ReasoningQuality: "unknown",
		ObservedAt: observedAt,
	}
	if event.Tokens.ReasoningTokens != 0 {
		tokens.ReasoningTokens = &reasoning
		tokens.ReasoningQuality = "measured"
	}
	return store.WriteTurnTokenUsage(ctx, tokens)
}

// TurnStorePath returns the canonical shared telemetry database path.
func TurnStorePath() (string, error) { return telemetry.DefaultDBPath() }

// TurnQuotaSourcePrefix returns the importer prefix useful for row parity
// diagnostics; it contains no file contents.
func TurnQuotaSourcePrefix(path string) string {
	h := sha256.Sum256([]byte(filepath.Clean(path)))
	return "turn-jsonl:" + hex.EncodeToString(h[:8]) + ":"
}

// TurnQuotaImportCount counts source events imported from one JSONL path.
func TurnQuotaImportCount(ctx context.Context, dbPath, path string) (int, error) {
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return 0, err
	}
	defer store.Close()
	prefix := TurnQuotaSourcePrefix(path)
	var count int
	err = store.QueryRow(ctx, `SELECT count(*) FROM turn_quota_boundaries WHERE substr(import_key,1,?)=?`, len(prefix), prefix).Scan(&count)
	return count, err
}

// TurnQuotaImportMarker checks whether a source path has completed import.
func TurnQuotaImportMarker(ctx context.Context, dbPath, path string) (bool, error) {
	store, err := openTurnUsageStore(ctx, dbPath)
	if err != nil {
		return false, err
	}
	defer store.Close()
	h := sha256.Sum256([]byte(filepath.Clean(path)))
	marker := "turn-jsonl-import-v1:" + hex.EncodeToString(h[:8])
	var count int
	err = store.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name=?`, marker).Scan(&count)
	return count > 0, err
}
