package usage

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// CaptureCompactQuota takes the one quota sample a Claude compaction records
// (issue 1303 M2). It runs in the collector, never in the hook process.
//
// Order: a cache younger than turn_quota.compact_cache_max_age is read and not
// refetched. An older cache is refreshed only under the cross-process live-fetch
// lock, taken once without waiting. If another process holds the lock, the
// cached sample is returned with its age and nothing is fetched. The fetch is
// bounded by turn_quota.timeout; a failed fetch is recorded as Error with no
// windows, and the caller's compaction rows are not touched.
func CaptureCompactQuota(ctx context.Context, now time.Time) TurnQuotaReading {
	_, maxAge, err := turnQuotaSpecValues()
	if err != nil {
		return TurnQuotaReading{CapturedAt: now.UTC(), Error: err.Error()}
	}
	dir, err := claudeQuotaDir()
	if err != nil {
		return TurnQuotaReading{CapturedAt: now.UTC(), Error: err.Error()}
	}
	cachePath := liveFetchCachePath(dir)

	cacheAt, cached, cacheErr := ProviderSnapshotFromStore(ctx, "claude")
	if cacheErr == nil && !cacheAt.IsZero() && now.Sub(cacheAt) < maxAge {
		return compactReadingFromCache("compact-cache", now, cacheAt, cached)
	}

	lock, locked := lockLiveFetchCacheOnce(cachePath)
	if !locked {
		if cacheErr == nil && !cacheAt.IsZero() {
			return compactReadingFromCache("compact-locked", now, cacheAt, cached)
		}
		return TurnQuotaReading{CapturedAt: now.UTC(), Source: "compact-locked", Error: "live quota lock held and no cached sample"}
	}
	defer unlockLiveFetchCache(lock)

	// Another process may have refreshed the cache while this one waited for
	// the lock; re-check so a finished refresh is not fetched twice.
	if cacheAt2, cached2, err2 := ProviderSnapshotFromStore(ctx, "claude"); err2 == nil && !cacheAt2.IsZero() && now.Sub(cacheAt2) < maxAge {
		return compactReadingFromCache("compact-cache", now, cacheAt2, cached2)
	}

	timeout, err := TurnQuotaTimeoutForProvider("claude")
	if err != nil {
		return TurnQuotaReading{CapturedAt: now.UTC(), Source: "compact-fetch", Error: err.Error()}
	}
	fetchCtx, cancel := context.WithTimeout(ForceQuotaFetch(ctx), timeout)
	defer cancel()
	fetchCtx = withLiveFetchLockHeld(fetchCtx)
	started := time.Now()
	u := CollectClaude(fetchCtx, dir, turnQuotaHTTPClientFactory(timeout))
	fetched := time.Now().UTC()

	reading := TurnQuotaReading{CapturedAt: fetched, Source: "compact-fetch", ProbeDurationMS: time.Since(started).Milliseconds()}
	if u.QuotaFetchError != "" {
		reading.Error = u.QuotaFetchError
		return reading
	}
	reading.HasCache = true
	reading.StoreWindows = AgentUsageToStoreWindows(u, fetched)
	reading.Windows = AgentUsageToQuotaHistoryEntries(u, fetched)
	if len(reading.Windows) == 0 {
		reading.Error = "no quota reading available"
	}
	return reading
}

func compactReadingFromCache(source string, now, cacheAt time.Time, u AgentUsage) TurnQuotaReading {
	return TurnQuotaReading{
		CapturedAt:   now.UTC(),
		CacheAgeMS:   max(now.Sub(cacheAt).Milliseconds(), 0),
		HasCache:     true,
		Source:       source,
		StoreWindows: AgentUsageToStoreWindows(u, cacheAt),
		Windows:      AgentUsageToQuotaHistoryEntries(u, cacheAt),
	}
}

// claudeQuotaDir is the Claude config directory the live fetch reads.
func claudeQuotaDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// RecordCompactQuota takes one quota sample with capture and stores it as a
// turn_quota_boundaries row with boundary "compact" and turn 0 (issue 1303 M2).
// recordID is the import key, so a replayed record stores its sample once. A
// failed read is stored as the row's error; no other table is written.
func RecordCompactQuota(ctx context.Context, dbPath, recordID, sessionID string, now time.Time, capture func(context.Context, time.Time) TurnQuotaReading) error {
	reading := capture(ctx, now)
	boundary := TurnQuotaBoundaryFromReading(sessionID, "claude", 0, "compact", reading)
	boundary.ImportKey = "compact-record:" + recordID
	return PersistTurnQuotaBoundary(ctx, dbPath, boundary)
}
