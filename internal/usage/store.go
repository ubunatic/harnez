package usage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

// StoreCompactSummary makes the existing compact collection result durable,
// then projects the store's normalized quota readings back into that result.
// Any database error leaves the collected summary untouched for safe fallback.
func StoreCompactSummary(ctx context.Context, homeDir string, summary UsageSummary) (UsageSummary, error) {
	return storeCompactSummary(ctx, homeDir, summary, "")
}

// StoreCompactSummaryAt is the testable form of StoreCompactSummary with an
// explicit telemetry database path.
func StoreCompactSummaryAt(ctx context.Context, homeDir string, summary UsageSummary, dbPath string) (UsageSummary, error) {
	return storeCompactSummary(ctx, homeDir, summary, dbPath)
}

// ImportUsageCompatibility backfills the existing quota snapshots/history
// once so consumers can read them through the shared store API.
func ImportUsageCompatibility(ctx context.Context, homeDir, dbPath string) error {
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return err
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return err
	}
	if err := archiveLegacySourcesBeforeImport(ctx, store); err != nil {
		return err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return err
	}
	return importCompatibility(store, ctx, homeDir)
}

func archiveLegacySourcesBeforeImport(ctx context.Context, store *usagestore.Store) error {
	if err := store.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var done int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name='legacy-mirror-archive-v1'`).Scan(&done); err != nil {
		return err
	}
	if done > 0 {
		return nil
	}
	result, err := ArchiveLegacyUsageData(DefaultLegacyUsageArchivePaths())
	if err != nil {
		return fmt.Errorf("archive legacy usage sources before compatibility import: %w", err)
	}
	if result.Files > 0 {
		dbPath, err := telemetry.DefaultDBPath()
		if err != nil {
			return err
		}
		if err := importUsageArchiveIntoStore(ctx, dbPath, store, result.Path); err != nil {
			return err
		}
	}
	return store.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES('legacy-mirror-archive-v1',?)`, time.Now().UTC().Format(time.RFC3339Nano))
}

func importUsageArchiveIntoStore(ctx context.Context, dbPath string, store *usagestore.Store, root string) error {
	data, err := os.ReadFile(filepath.Join(root, legacyUsageArchiveManifest))
	if err != nil {
		return err
	}
	var manifest LegacyUsageArchiveManifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != 1 {
		return fmt.Errorf("usage archive: invalid manifest")
	}
	for _, record := range manifest.Files {
		path := filepath.Join(root, filepath.Clean(record.ArchivePath))
		switch record.Kind {
		case "provider-cache":
			if err := importArchivedProviderCache(ctx, store, path); err != nil {
				return err
			}
		case "state-snapshot":
			if err := importArchivedStateSnapshot(ctx, store, path); err != nil {
				return err
			}
		case "usage-history":
			if err := importArchivedHistory(ctx, store, path); err != nil {
				return err
			}
		case "turn-quota-jsonl":
			if _, err := ImportTurnQuotaJSONL(ctx, dbPath, path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("usage archive: unsupported source kind %q", record.Kind)
		}
	}
	return nil
}

// QuotaHistoryFromStore returns the store's historical window view in the
// legacy report shape used by host-session fitted attribution.
func QuotaHistoryFromStore(ctx context.Context, homeDir, dbPath string) ([]QuotaHistoryEntry, error) {
	if err := ImportUsageCompatibility(ctx, homeDir, dbPath); err != nil {
		return nil, err
	}
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
	defer store.Close()
	windows, err := store.QuotaHistory(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]QuotaHistoryEntry, 0, len(windows))
	for _, window := range windows {
		used := int(math.Round(window.UsedFraction * 100))
		remaining := max(100-used, 0)
		out = append(out, QuotaHistoryEntry{Timestamp: window.ObservedAt, Agent: window.Provider, Group: window.Pool, Window: window.Name, UsedPercent: used, RemainingPercent: remaining, ResetAt: window.ResetAt})
	}
	return out, nil
}

// UsageHistoryFromStore returns complete compatibility snapshots through the
// telemetry store after importing any retained legacy JSONL files.
func UsageHistoryFromStore(ctx context.Context, homeDir, dbPath string) ([]HistoryEntry, error) {
	if err := ImportUsageCompatibility(ctx, homeDir, dbPath); err != nil {
		return nil, err
	}
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
	defer store.Close()
	rows, err := store.UsageSummaries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, 0, len(rows))
	for _, row := range rows {
		var summary UsageSummary
		if err := json.Unmarshal(row.Payload, &summary); err != nil {
			return nil, err
		}
		if summary.Timestamp.IsZero() {
			summary.Timestamp = row.ObservedAt
		}
		out = append(out, HistoryEntry{Hostname: row.Hostname, UsageSummary: summary})
	}
	return out, nil
}

// ProviderSnapshotFromStore projects the best current quota windows for a
// provider into the legacy collector model without reading its JSON cache.
func ProviderSnapshotFromStore(ctx context.Context, provider string) (time.Time, AgentUsage, error) {
	return ProviderSnapshotFromStoreAt(ctx, "", provider)
}

func ProviderSnapshotFromStoreAt(ctx context.Context, dbPath, provider string) (time.Time, AgentUsage, error) {
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return time.Time{}, AgentUsage{}, err
		}
	}
	if err := ImportUsageCompatibility(ctx, "", dbPath); err != nil {
		return time.Time{}, AgentUsage{}, err
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return time.Time{}, AgentUsage{}, err
	}
	defer store.Close()
	at, windows, err := store.LatestProviderSnapshot(ctx, provider)
	if err != nil {
		return time.Time{}, AgentUsage{}, err
	}
	usage := AgentUsage{AgentID: provider, LastRefreshed: at}
	applyStoredWindows(&usage, windows, time.Now())
	return at, usage, nil
}

func storeProviderSnapshot(ctx context.Context, provider string, at time.Time, usage AgentUsage) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return err
	}
	return store.WriteCurrent(ctx, at, AgentUsageToStoreWindows(usage, at))
}

func appendQuotaHistoryToStore(entries []QuotaHistoryEntry, throttle time.Duration, now time.Time) error {
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	ctx := context.Background()
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return err
	}
	if err := importCompatibility(store, ctx, ""); err != nil {
		return err
	}
	current, err := store.QuotaHistory(ctx)
	if err != nil {
		return err
	}
	last := map[string]usagestore.Window{}
	for _, window := range current {
		key := window.Provider + "|" + window.Pool + "|" + window.Key
		if old, ok := last[key]; !ok || window.ObservedAt.After(old.ObservedAt) {
			last[key] = window
		}
	}
	if len(last) == 0 {
		legacy, err := readQuotaHistoryFile(HistoryDir(""))
		if err != nil {
			return err
		}
		for _, entry := range legacy {
			key := entry.Agent + "|" + entry.Group + "|" + usagestore.NormalizeWindowKey(entry.Agent, entry.Group, entry.Window)
			window := usagestore.Window{Provider: entry.Agent, Pool: entry.Group, Key: usagestore.NormalizeWindowKey(entry.Agent, entry.Group, entry.Window), Name: entry.Window, UsedFraction: float64(entry.UsedPercent) / 100, ResetAt: entry.ResetAt, ObservedAt: entry.Timestamp}
			if old, ok := last[key]; !ok || window.ObservedAt.After(old.ObservedAt) {
				last[key] = window
			}
		}
	}
	var windows []usagestore.Window
	for _, entry := range entries {
		windowKey := usagestore.NormalizeWindowKey(entry.Agent, entry.Group, entry.Window)
		key := entry.Agent + "|" + entry.Group + "|" + windowKey
		old, ok := last[key]
		changed := !ok || math.Round(old.UsedFraction*100) != float64(entry.UsedPercent) || (old.ResetAt == nil) != (entry.ResetAt == nil) || (old.ResetAt != nil && entry.ResetAt != nil && !old.ResetAt.Equal(*entry.ResetAt))
		elapsed := ok && throttle > 0 && entry.Timestamp.Sub(old.ObservedAt) >= throttle
		if !changed && !elapsed {
			continue
		}
		windows = append(windows, usagestore.Window{Provider: entry.Agent, Pool: entry.Group, Key: windowKey, Name: entry.Window, Source: "provider-api", Freshness: "fresh", UsedFraction: float64(entry.UsedPercent) / 100, ResetAt: entry.ResetAt, ObservedAt: entry.Timestamp})
	}
	if len(windows) > 0 {
		if now.IsZero() {
			now = time.Now()
		}
		if err := store.WriteCurrent(ctx, now, windows); err != nil {
			return err
		}
	}
	return writeQuotaHistoryMirror(ctx, store, HistoryDir(""))
}

func writeQuotaHistoryMirror(ctx context.Context, store *usagestore.Store, dir string) error {
	entries, err := QuotaHistoryFromStore(ctx, "", "")
	if err != nil {
		return err
	}
	var out strings.Builder
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := archiveLegacySourcesBeforeImport(ctx, store); err != nil {
		return err
	}
	tmp := QuotaHistoryPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, QuotaHistoryPath(dir))
}

func writeProviderSnapshotMirror(path string, fetchedAt time.Time, provider string, usage AgentUsage) error {
	if err := ensureLegacyArchiveBeforeMirror(); err != nil {
		return err
	}
	var payload any
	switch provider {
	case "claude":
		payload = claudeQuotaPayload{Session: usage.Session, Weekly: usage.Weekly}
	case "codex":
		payload = codexQuotaPayload{Session: usage.Session, Weekly: usage.Weekly}
	case "agy":
		payload = agyQuotaPayload{ModelGroups: usage.ModelGroups}
	default:
		return fmt.Errorf("unsupported provider cache mirror %q", provider)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(liveFetchCache[any]{FetchedAt: fetchedAt, Payload: payload})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func appendUsageSummaryToStore(dir string, summary UsageSummary) error {
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	ctx := context.Background()
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return err
	}
	if err := archiveLegacySourcesBeforeImport(ctx, store); err != nil {
		return err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return err
	}
	if err := importCompatibility(store, ctx, ""); err != nil {
		return err
	}
	if summary.Timestamp.IsZero() {
		summary.Timestamp = time.Now().UTC()
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	payload, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	if err := store.WriteUsageSummary(ctx, usagestore.UsageSummaryRecord{Hostname: host, ObservedAt: summary.Timestamp, Payload: payload}); err != nil {
		return err
	}
	if err := archiveBeforeMirrorOverwrite(ctx, store); err != nil {
		return err
	}
	return writeUsageHistoryMirrors(ctx, store, dir)
}

func archiveBeforeMirrorOverwrite(ctx context.Context, store *usagestore.Store) error {
	if err := store.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var done int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name='legacy-mirror-archive-v1'`).Scan(&done); err != nil {
		return err
	}
	if done > 0 {
		return nil
	}
	if _, err := ArchiveLegacyUsageData(DefaultLegacyUsageArchivePaths()); err != nil {
		return fmt.Errorf("archive legacy usage sources before mirror generation: %w", err)
	}
	return store.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES('legacy-mirror-archive-v1',?)`, time.Now().UTC().Format(time.RFC3339Nano))
}

func ensureLegacyArchiveBeforeMirror() error {
	if _, err := ArchiveLegacyUsageData(DefaultLegacyUsageArchivePaths()); err != nil {
		return fmt.Errorf("archive legacy usage sources before mirror generation: %w", err)
	}
	return nil
}

func writeUsageHistoryMirrors(ctx context.Context, store *usagestore.Store, dir string) error {
	records, err := store.UsageSummaries(ctx)
	if err != nil {
		return err
	}
	byHost := map[string]*strings.Builder{}
	for _, record := range records {
		var summary UsageSummary
		if err := json.Unmarshal(record.Payload, &summary); err != nil {
			return err
		}
		line, err := json.Marshal(HistoryEntry{Hostname: record.Hostname, UsageSummary: summary})
		if err != nil {
			return err
		}
		if byHost[record.Hostname] == nil {
			byHost[record.Hostname] = &strings.Builder{}
		}
		byHost[record.Hostname].Write(line)
		byHost[record.Hostname].WriteByte('\n')
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := ensureLegacyArchiveBeforeMirror(); err != nil {
		return err
	}
	if err := archiveBeforeMirrorOverwrite(ctx, store); err != nil {
		return err
	}
	for host, body := range byHost {
		path := filepath.Join(dir, sanitizeHostname(host)+".jsonl")
		lock, ok := lockHistoryFile(path)
		if !ok {
			return fmt.Errorf("lock generated history mirror %s", path)
		}
		tmp := path + ".generated.tmp"
		err := os.WriteFile(tmp, []byte(body.String()), 0o600)
		if err == nil {
			err = os.Rename(tmp, path)
		}
		unlockHistoryFile(lock)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(body.String()))
		marker := "generated-history:" + filepath.Clean(path) + ":" + hex.EncodeToString(sum[:])
		if err := store.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES(?,?)`, marker, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return nil
}

func storeCompactSummary(ctx context.Context, homeDir string, summary UsageSummary, dbPath string) (UsageSummary, error) {
	if dbPath == "" {
		var err error
		dbPath, err = telemetry.DefaultDBPath()
		if err != nil {
			return summary, err
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return summary, err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, q string) error { return store.Exec(ctx, q) }); err != nil {
		return summary, err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return summary, err
	}
	if err := importCompatibility(store, ctx, homeDir); err != nil {
		return summary, err
	}
	now := summary.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	for _, agent := range summary.Agents {
		at := now
		if !agent.LastRefreshed.IsZero() {
			at = agent.LastRefreshed
		}
		if err := importAgent(store, ctx, agent.AgentID, "collect-all", at, agent); err != nil {
			return summary, err
		}
	}
	for i := range summary.Agents {
		readings, err := store.CurrentFor(ctx, summary.Agents[i].AgentID)
		if err != nil {
			return summary, err
		}
		applyStoredWindows(&summary.Agents[i], readings, now)
	}
	return summary, nil
}

func applyStoredWindows(agent *AgentUsage, readings []usagestore.Window, now time.Time) {
	if len(readings) == 0 {
		return
	}
	agent.Session, agent.Weekly = nil, nil
	agent.ModelGroups = nil
	for _, r := range readings {
		if r.InputTokens != nil || r.OutputTokens != nil || r.CachedInputTokens != nil {
			tokens := &TokenBreakdown{}
			if r.InputTokens != nil {
				tokens.InputTokens = *r.InputTokens
			}
			if r.OutputTokens != nil {
				tokens.OutputTokens = *r.OutputTokens
			}
			if r.CachedInputTokens != nil {
				tokens.CacheReadTokens = *r.CachedInputTokens
			}
			tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens
			agent.Tokens = tokens
		}
		if r.Pool == "tokens" {
			continue
		}
	}
	groups := map[string]int{}
	for _, r := range readings {
		reset := r.ResetAt
		w := QuotaWindow{Name: r.Name, Source: r.Source, UsedPercent: r.UsedFraction * 100, RemainingPercent: 100 - r.UsedFraction*100, ResetAt: reset}
		if reset != nil {
			w.DurationLeft = reset.Sub(now)
			if w.DurationLeft < 0 {
				w.DurationLeft = 0
			}
		}
		if r.Freshness == "stale" || now.Sub(r.ObservedAt) > DefaultCacheStaleness {
			w.Name = staleName(w.Name)
		}
		if w.ExpiredAt(now) {
			w.UsedPercent = 0
			w.RemainingPercent = 100
		}
		if r.Pool != "" {
			idx, ok := groups[r.Pool]
			if !ok {
				idx = len(agent.ModelGroups)
				groups[r.Pool] = idx
				agent.ModelGroups = append(agent.ModelGroups, ModelGroup{Name: r.Pool})
			}
			agent.ModelGroups[idx].Windows = append(agent.ModelGroups[idx].Windows, w)
			continue
		}
		name := strings.ToLower(r.Name)
		if r.Key == "weekly" || strings.Contains(name, "week") || strings.Contains(name, "7-day") {
			agent.Weekly = &w
		} else if agent.Session == nil {
			agent.Session = &w
		} else if agent.Weekly == nil {
			agent.Weekly = &w
		} else {
			if agent.ExtraWindows == nil {
				agent.ExtraWindows = map[string]QuotaWindow{}
			}
			agent.ExtraWindows[r.Key] = w
		}
	}
}

func staleName(name string) string {
	if strings.Contains(name, "(stale)") {
		return name
	}
	return strings.TrimSpace(name) + " (stale)"
}

// ImportCompatibility performs a one-time backfill, gated by an import marker.
func importCompatibility(s *usagestore.Store, ctx context.Context, homeDir string) error {
	if err := importUsageSummaryHistory(s, ctx, homeDir); err != nil {
		return err
	}
	if err := s.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var count int
	if err := s.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name='compat-v1'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var existing int
	if err := s.QueryRow(ctx, `SELECT count(*) FROM quota_windows`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return s.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES('compat-v1',?)`, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if err := archiveLegacySourcesBeforeImport(ctx, s); err != nil {
		return err
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	for _, id := range []string{"claude", "codex", "agy"} {
		if snap, err := readAgentSnapshotFile(StateDir(homeDir), id); err == nil && snap != nil {
			if err := importAgent(s, ctx, id, "state-snapshot", snap.FetchedAt, snap.Usage); err != nil {
				return err
			}
		}
	}
	for _, item := range []struct{ id, dir string }{
		{"claude", filepath.Join(homeDir, ".claude")}, {"codex", filepath.Join(homeDir, ".codex")}, {"agy", filepath.Join(homeDir, ".gemini", "antigravity-cli")},
	} {
		var raw any
		switch item.id {
		case "claude":
			raw = readLiveFetchCache[claudeQuotaPayload](liveFetchCachePath(item.dir))
		case "codex":
			raw = readLiveFetchCache[codexQuotaPayload](liveFetchCachePath(item.dir))
		case "agy":
			raw = readLiveFetchCache[agyQuotaPayload](liveFetchCachePath(item.dir))
		}
		data, _ := json.Marshal(raw)
		var cache struct {
			FetchedAt time.Time       `json:"fetched_at"`
			Payload   json.RawMessage `json:"payload"`
		}
		if len(data) > 0 && json.Unmarshal(data, &cache) == nil && cache.FetchedAt.IsZero() == false {
			u := AgentUsage{AgentID: item.id, LastRefreshed: cache.FetchedAt}
			switch item.id {
			case "claude":
				var p claudeQuotaPayload
				if json.Unmarshal(cache.Payload, &p) == nil {
					u.Session, u.Weekly = p.Session, p.Weekly
				}
			case "codex":
				var p codexQuotaPayload
				if json.Unmarshal(cache.Payload, &p) == nil {
					u.Session, u.Weekly = p.Session, p.Weekly
				}
			case "agy":
				var p agyQuotaPayload
				if json.Unmarshal(cache.Payload, &p) == nil {
					u.ModelGroups = p.ModelGroups
				}
			}
			if err := importAgent(s, ctx, item.id, "provider-cache", cache.FetchedAt, u); err != nil {
				return err
			}
		}
	}
	entries, err := readQuotaHistoryFile(HistoryDir(homeDir))
	if err != nil {
		return err
	}
	for _, e := range entries {
		w := QuotaWindow{Name: e.Window, UsedPercent: float64(e.UsedPercent), RemainingPercent: float64(e.RemainingPercent), ResetAt: e.ResetAt}
		u := AgentUsage{AgentID: e.Agent, LastRefreshed: e.Timestamp}
		if e.Group != "" {
			u.ModelGroups = []ModelGroup{{Name: e.Group, Windows: []QuotaWindow{w}}}
		} else if strings.Contains(strings.ToLower(e.Window), "week") {
			u.Weekly = &w
		} else {
			u.Session = &w
		}
		if err := importAgent(s, ctx, e.Agent, "history", e.Timestamp, u); err != nil {
			return err
		}
	}
	return s.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES('compat-v1',?)`, time.Now().UTC().Format(time.RFC3339Nano))
}

func importUsageSummaryHistory(s *usagestore.Store, ctx context.Context, homeDir string) error {
	if err := s.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return err
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	directories := []string{HistoryDir(homeDir), filepath.Join(homeDir, ".claude", "harnez", historyDirName)}
	seen := map[string]bool{}
	for _, dir := range directories {
		matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		if err != nil {
			return err
		}
		for _, path := range matches {
			if filepath.Base(path) == QuotaHistoryFilename || seen[path] {
				continue
			}
			seen[path] = true
			file, err := os.Open(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				file.Close()
				return err
			}
			sum := sha256.Sum256(contents)
			generatedMarker := "generated-history:" + filepath.Clean(path) + ":" + hex.EncodeToString(sum[:])
			var generated int
			if err := s.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name=?`, generatedMarker).Scan(&generated); err != nil {
				file.Close()
				return err
			}
			if generated > 0 {
				file.Close()
				continue
			}
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
			lineNumber := 0
			for scanner.Scan() {
				lineNumber++
				line := append([]byte(nil), scanner.Bytes()...)
				var entry HistoryEntry
				if json.Unmarshal(line, &entry) != nil || entry.Timestamp.IsZero() {
					continue
				}
				if entry.Hostname == "" {
					entry.Hostname = "unknown-host"
				}
				payload, err := json.Marshal(entry.UsageSummary)
				if err != nil {
					file.Close()
					return err
				}
				lineHash := sha256.Sum256(line)
				source := "legacy-history:" + hex.EncodeToString(lineHash[:]) + ":" + strconv.Itoa(lineNumber)
				if err := s.WriteUsageSummary(ctx, usagestore.UsageSummaryRecord{Hostname: entry.Hostname, ObservedAt: entry.Timestamp, Payload: payload, RecordKey: source}); err != nil {
					file.Close()
					return err
				}
			}
			scanErr := scanner.Err()
			closeErr := file.Close()
			if scanErr != nil {
				return scanErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}

func importAgent(s *usagestore.Store, ctx context.Context, provider, source string, at time.Time, agent AgentUsage) error {
	windows := make([]usagestore.Window, 0)
	freshness := "stale"
	if source == "collect-all" || source == "registry" {
		freshness = "fresh"
		if agent.IsValueStale() || time.Since(at) > DefaultCacheStaleness {
			freshness = "stale"
		}
	}
	add := func(pool string, w QuotaWindow) {
		key := stableWindowKey(provider, pool, w.Name)
		windowSource := source
		if w.Source != "" {
			windowSource = w.Source
		}
		name := cleanWindowName(w.Name)
		windows = append(windows, usagestore.Window{Provider: provider, Pool: pool, Key: key, Name: name, Source: windowSource, Freshness: freshness, UsedFraction: w.UsedPercent / 100, ResetAt: w.ResetAt, ObservedAt: at})
	}
	if agent.Tokens != nil {
		input, cached, output := agent.Tokens.InputTokens, agent.Tokens.CacheReadTokens, agent.Tokens.OutputTokens
		windows = append(windows, usagestore.Window{Provider: provider, Pool: "tokens", Key: "cumulative", Name: "tokens", Source: source, Freshness: freshness, UsedFraction: 0, InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output, ObservedAt: at})
	}
	if agent.Session != nil {
		add("", *agent.Session)
	}
	if agent.Weekly != nil {
		add("", *agent.Weekly)
	}
	for _, g := range agent.ModelGroups {
		for _, w := range g.Windows {
			add(g.Name, w)
		}
	}
	for _, w := range agent.ExtraWindows {
		add("", w)
	}
	return s.WriteCurrent(ctx, at, windows)
}

// AgentUsageToStoreWindows preserves fractional quota values and source
// attribution for a boundary capture before compatibility projections round
// percentages for display.
func AgentUsageToStoreWindows(agent AgentUsage, observedAt time.Time) []usagestore.Window {
	provider := agent.AgentID
	freshness := "fresh"
	if agent.IsValueStale() {
		freshness = "stale"
	}
	var windows []usagestore.Window
	add := func(pool string, quota QuotaWindow) {
		name := cleanWindowName(quota.Name)
		windowFreshness := freshness
		if strings.Contains(strings.ToLower(quota.Name), "(stale)") {
			windowFreshness = "stale"
		}
		source := quota.Source
		if source == "" {
			source = "turn-capture"
		}
		at := observedAt
		if at.IsZero() {
			at = time.Now().UTC()
		}
		windows = append(windows, usagestore.Window{
			Provider: provider, Pool: pool, Key: usagestore.NormalizeWindowKey(provider, "", name), Name: name,
			Source: source, Freshness: windowFreshness, UsedFraction: quota.UsedPercent / 100,
			ResetAt: quota.ResetAt, ObservedAt: at,
		})
	}
	if agent.Session != nil {
		add("", *agent.Session)
	}
	if agent.Weekly != nil {
		add("", *agent.Weekly)
	}
	for _, group := range agent.ModelGroups {
		for _, quota := range group.Windows {
			add(group.Name, quota)
		}
	}
	for _, quota := range agent.ExtraWindows {
		add("", quota)
	}
	return windows
}

func cleanWindowName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, " (stale)", ""), " (STALE)", ""))
}

func stableWindowKey(provider, pool, name string) string {
	return usagestore.NormalizeWindowKey(provider, pool, cleanWindowName(name))
}
