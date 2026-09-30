package usage

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
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
	return importCompatibility(store, ctx, homeDir)
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
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	for _, id := range []string{"claude", "codex", "agy"} {
		if snap, err := ReadAgentSnapshot(StateDir(homeDir), id); err == nil && snap != nil {
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
	entries, err := ReadQuotaHistory(HistoryDir(homeDir))
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
