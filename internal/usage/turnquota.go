package usage

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TurnQuotaReading is a bounded, best-effort quota observation for one agent turn.
type TurnQuotaReading struct {
	CapturedAt time.Time           `json:"captured_at"`
	CacheAgeMS int64               `json:"cache_age_ms"`
	HasCache   bool                `json:"has_cache"`
	Windows    []QuotaHistoryEntry `json:"windows,omitempty"`
	Error      string              `json:"error,omitempty"`
}

const TurnQuotaTimeout = 1800 * time.Millisecond

// CaptureTurnQuota observes a provider's quota, using the regular shared cache
// for ordinary captures and bypassing its freshness gate when force is true.
// Errors are returned as data so a quota problem cannot fail the agent turn.
func CaptureTurnQuota(ctx context.Context, provider string, force bool) TurnQuotaReading {
	ctx, cancel := context.WithTimeout(ctx, TurnQuotaTimeout)
	defer cancel()
	if force {
		ctx = ForceQuotaFetch(ctx)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return TurnQuotaReading{CapturedAt: time.Now().UTC(), Error: err.Error()}
	}
	client := &http.Client{Timeout: TurnQuotaTimeout}
	var u AgentUsage
	var cachePath string
	switch provider {
	case "claude":
		dir := filepath.Join(home, ".claude")
		cachePath = liveFetchCachePath(dir)
		u = CollectClaude(ctx, dir, client)
	case "codex":
		dir := filepath.Join(home, ".codex")
		cachePath = liveFetchCachePath(dir)
		u = CollectCodex(ctx, dir, client)
	case "agy":
		dir := filepath.Join(home, ".gemini", "antigravity-cli")
		cachePath = liveFetchCachePath(dir)
		u = CollectAGY(ctx, dir, client)
	default:
		return TurnQuotaReading{CapturedAt: time.Now().UTC(), Error: "unsupported quota provider: " + provider}
	}

	now := time.Now().UTC()
	reading := TurnQuotaReading{CapturedAt: now, Error: u.QuotaFetchError}
	if cache := readProviderQuotaCache(provider, cachePath); !cache.IsZero() {
		reading.HasCache = true
		reading.CacheAgeMS = max(now.Sub(cache).Milliseconds(), 0)
	}
	reading.Windows = AgentUsageToQuotaHistoryEntries(u, now)
	if len(reading.Windows) == 0 && reading.Error == "" {
		reading.Error = "no quota reading available"
	}
	return reading
}

func readProviderQuotaCache(provider, path string) time.Time {
	switch provider {
	case "claude":
		if c := readLiveFetchCache[claudeQuotaPayload](path); c != nil {
			return c.FetchedAt
		}
	case "codex":
		if c := readLiveFetchCache[codexQuotaPayload](path); c != nil {
			return c.FetchedAt
		}
	case "agy":
		if c := readLiveFetchCache[agyQuotaPayload](path); c != nil {
			return c.FetchedAt
		}
	}
	return time.Time{}
}
