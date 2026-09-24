package usage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TurnQuotaReading is a bounded, best-effort quota observation for one agent turn.
type TurnQuotaReading struct {
	CapturedAt      time.Time           `json:"captured_at"`
	CacheAgeMS      int64               `json:"cache_age_ms"`
	HasCache        bool                `json:"has_cache"`
	ProbeDurationMS int64               `json:"probe_duration_ms,omitempty"`
	Windows         []QuotaHistoryEntry `json:"windows,omitempty"`
	Error           string              `json:"error,omitempty"`
}

const TurnQuotaTimeout = 1800 * time.Millisecond
const AGYTurnQuotaTimeout = agyUsageCmdTimeout + 2*time.Second

// TurnQuotaTimeoutForProvider returns the bounded budget for a turn-boundary
// quota reading. AGY's local /usage command has its own longer process bound.
func TurnQuotaTimeoutForProvider(provider string) time.Duration {
	if provider == "agy" {
		return AGYTurnQuotaTimeout
	}
	return TurnQuotaTimeout
}

var turnQuotaHTTPClientFactory = func() *http.Client { return &http.Client{Timeout: TurnQuotaTimeout} }

// CaptureTurnQuota observes a provider's quota, using the regular shared cache
// for ordinary captures and bypassing its freshness gate when force is true.
// Errors are returned as data so a quota problem cannot fail the agent turn.
func CaptureTurnQuota(ctx context.Context, provider string, force bool) TurnQuotaReading {
	return CaptureTurnQuotaSince(ctx, provider, force, time.Time{})
}

// CaptureTurnQuotaSince observes quota and forces a provider fetch when its
// shared cache predates turnStarted. Ordinary cache freshness rules still
// apply when force is false and the cache was refreshed during this turn.
func CaptureTurnQuotaSince(ctx context.Context, provider string, force bool, turnStarted time.Time) TurnQuotaReading {
	return CaptureTurnQuotaSinceCache(ctx, provider, force, turnStarted, time.Time{})
}

// CaptureTurnQuotaSinceCache also receives the baseline cache timestamp taken
// before the provider turn started, preventing another process's later cache
// write from being mistaken for a measurement made during this turn.
func CaptureTurnQuotaSinceCache(ctx context.Context, provider string, force bool, turnStarted, baselineCacheAt time.Time) TurnQuotaReading {
	ctx, cancel := context.WithTimeout(ctx, TurnQuotaTimeoutForProvider(provider))
	defer cancel()
	cachePath, err := providerQuotaCachePath(provider)
	if err != nil {
		return TurnQuotaReading{CapturedAt: time.Now().UTC(), Error: err.Error()}
	}
	if !force && !turnStarted.IsZero() {
		fetchedAt := readProviderQuotaCache(provider, cachePath)
		if fetchedAt.Before(turnStarted) || (!baselineCacheAt.IsZero() && !fetchedAt.After(baselineCacheAt)) {
			force = true
		}
	}
	if force {
		ctx = ForceQuotaFetch(ctx)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return TurnQuotaReading{CapturedAt: time.Now().UTC(), Error: err.Error()}
	}
	client := turnQuotaHTTPClientFactory()
	var u AgentUsage
	switch provider {
	case "claude":
		dir := filepath.Join(home, ".claude")
		u = CollectClaude(ctx, dir, client)
	case "codex":
		dir := filepath.Join(home, ".codex")
		u = CollectCodex(ctx, dir, client)
	case "agy":
		dir := filepath.Join(home, ".gemini", "antigravity-cli")
		u = CollectAGY(ctx, dir, client)
	}

	now := time.Now().UTC()
	reading := TurnQuotaReading{CapturedAt: now, Error: u.QuotaFetchError, ProbeDurationMS: u.QuotaFetchDurationMS}
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

func providerQuotaCachePath(provider string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var dir string
	switch provider {
	case "claude":
		dir = filepath.Join(home, ".claude")
	case "codex":
		dir = filepath.Join(home, ".codex")
	case "agy":
		dir = filepath.Join(home, ".gemini", "antigravity-cli")
	default:
		return "", fmt.Errorf("unsupported quota provider: %s", provider)
	}
	return liveFetchCachePath(dir), nil
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
