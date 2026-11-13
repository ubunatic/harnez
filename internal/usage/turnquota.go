package usage

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

// TurnQuotaReading is a bounded, best-effort quota observation for one agent turn.
type TurnQuotaReading struct {
	CapturedAt                   time.Time           `json:"captured_at"`
	CacheAgeMS                   int64               `json:"cache_age_ms"`
	HasCache                     bool                `json:"has_cache"`
	ProbeDurationMS              int64               `json:"probe_duration_ms,omitempty"`
	Source                       string              `json:"source,omitempty"`
	StoreWindows                 []usagestore.Window `json:"store_windows,omitempty"`
	Windows                      []QuotaHistoryEntry `json:"windows,omitempty"`
	SubscriptionPriceShareMicros *int64              `json:"subscription_price_share_micros,omitempty"`
	Error                        string              `json:"error,omitempty"`
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
	reading := TurnQuotaReading{CapturedAt: now, Error: u.QuotaFetchError, ProbeDurationMS: u.QuotaFetchDurationMS, Source: "turn-capture", StoreWindows: AgentUsageToStoreWindows(u, now)}
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
	_ = path // retained in the signature for callers using a custom provider directory
	at, _, err := ProviderSnapshotFromStore(context.Background(), provider)
	if err != nil {
		return time.Time{}
	}
	return at
}

type quotaWindowSample struct {
	pool, key string
	used      float64
	resetAt   *time.Time
}

func extractQuotaSamples(r TurnQuotaReading) []quotaWindowSample {
	if len(r.StoreWindows) > 0 {
		out := make([]quotaWindowSample, 0, len(r.StoreWindows))
		for _, w := range r.StoreWindows {
			if strings.EqualFold(w.Pool, "tokens") {
				continue
			}
			key := w.Key
			if key == "" {
				key = usagestore.NormalizeWindowKey(w.Provider, w.Pool, w.Name)
			}
			out = append(out, quotaWindowSample{pool: w.Pool, key: key, used: w.UsedFraction, resetAt: w.ResetAt})
		}
		return out
	}
	out := make([]quotaWindowSample, 0, len(r.Windows))
	for _, w := range r.Windows {
		key := usagestore.NormalizeWindowKey(w.Agent, w.Group, w.Window)
		out = append(out, quotaWindowSample{pool: w.Group, key: key, used: float64(w.UsedPercent) / 100.0, resetAt: w.ResetAt})
	}
	return out
}

func sameResetMarker(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func isWeeklyQuotaKey(key string) bool {
	return key == "weekly" || key == "seven_day"
}

// ComputeTurnQuotaShare compares non-error before/after turn quota readings and
// returns the matched window name, before/after usage percentages, and the
// prorated weekly subscription share in micro-USD when a weekly window moved
// monotonically without reset and monthlyUSDMicros is known (> 0).
// Unknown or uncomparable inputs keep share as nil (never 0).
func ComputeTurnQuotaShare(before, after TurnQuotaReading, monthlyUSDMicros *int64) (share *int64, window string, beforePct, afterPct *float64) {
	if before.Error != "" || after.Error != "" || before.CapturedAt.IsZero() || !after.CapturedAt.After(before.CapturedAt) {
		return nil, "", nil, nil
	}
	beforeSamples := extractQuotaSamples(before)
	afterSamples := extractQuotaSamples(after)
	type pair struct {
		before, after quotaWindowSample
	}
	var weeklyMatch *pair
	var fallbackMatch *pair
	for _, b := range beforeSamples {
		for _, a := range afterSamples {
			if a.pool != b.pool || a.key != b.key {
				continue
			}
			if isWeeklyQuotaKey(b.key) && weeklyMatch == nil {
				p := pair{before: b, after: a}
				weeklyMatch = &p
			} else if fallbackMatch == nil {
				p := pair{before: b, after: a}
				fallbackMatch = &p
			}
		}
	}
	chosen := weeklyMatch
	if chosen == nil {
		chosen = fallbackMatch
	}
	if chosen == nil {
		return nil, "", nil, nil
	}
	bPct := math.Round(chosen.before.used*10000) / 100
	aPct := math.Round(chosen.after.used*10000) / 100
	windowName := chosen.before.key
	if chosen.before.pool != "" {
		windowName = chosen.before.pool + ":" + chosen.before.key
	}
	if !isWeeklyQuotaKey(chosen.before.key) || !sameResetMarker(chosen.before.resetAt, chosen.after.resetAt) || chosen.after.used < chosen.before.used {
		return nil, windowName, &bPct, &aPct
	}
	if monthlyUSDMicros == nil || *monthlyUSDMicros <= 0 {
		return nil, windowName, &bPct, &aPct
	}
	weeklyMicros := (*monthlyUSDMicros * 12) / 52
	delta := chosen.after.used - chosen.before.used
	val := int64(math.Round(delta * float64(weeklyMicros)))
	return &val, windowName, &bPct, &aPct
}
