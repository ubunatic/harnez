package usage

import (
	"os"
	"path/filepath"
	"time"
)

// fetchDurationsFilename is the on-disk cache of learned fetch-duration
// estimates (issue 168). It reuses livefetchcache.go's generic
// liveFetchCache[T]/flock machinery rather than inventing a new storage
// mechanism -- readLiveFetchCache/writeLiveFetchCache/lockLiveFetchCache/
// lockLiveFetchInProcess already implement exactly the "torn-write-proof,
// cross-process-safe JSON sidecar" shape this needs. Unlike the per-agent
// quota caches those were built for (which live inside an agent's own
// config dir, e.g. ~/.claude), this cache isn't tied to any one agent, so
// it lives under the shared harnez state root instead -- the same
// XDG-resolved base StateDir uses for agents/usage, one level up.
const fetchDurationsFilename = "fetch-durations.json"

// fetchDurationEWMAAlpha weights each newly observed fetch duration against
// the prior persisted estimate. 0.3 lets a real, sustained change in
// network/host conditions show up within a handful of fetches without
// letting a single slow/fast outlier swing the estimate wildly -- a
// straightforward EWMA per the ticket's "don't over-build this" guidance,
// not a full statistics subsystem.
const fetchDurationEWMAAlpha = 0.3

// fetchDurationSample is one fetch kind's persisted rolling estimate.
type fetchDurationSample struct {
	EstimateMS int64 `json:"estimate_ms"`
	Samples    int   `json:"samples"`
}

// fetchDurationKindLocal names the local CollectAll fetch kind. Remote
// fetches are keyed per-host (see fetchDurationKindForHost) since a
// CollectRemote call over SSH to one host has no reason to share an
// estimate with a local fetch or with a different remote host -- issue
// 168's motivating finding is exactly that these durations differ.
const fetchDurationKindLocal = "local"

// fetchDurationKindForHost returns the fetch-duration cache key for a
// RunWatchWithOptions fetch targeting host ("" for the local CollectAll
// path, otherwise the CollectRemote host).
func fetchDurationKindForHost(host string) string {
	if host == "" {
		return fetchDurationKindLocal
	}
	return "remote:" + host
}

// fetchDurationsPath resolves the shared fetch-duration estimate cache
// file, using the same XDG Base Directory resolution StateDir uses, one
// directory above its agents/usage subtree.
func fetchDurationsPath(homeDir string) string {
	if xdg := os.Getenv(stateCacheDirEnv); xdg != "" {
		return filepath.Join(xdg, stateAppDir, fetchDurationsFilename)
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return filepath.Join(homeDir, ".local", "state", stateAppDir, fetchDurationsFilename)
}

// loadFetchDurationEstimate returns the persisted rolling estimate for
// kind, or (0, false) if none exists yet -- a true cold start (first-ever
// run, or first fetch of this particular kind on this host). Callers must
// treat ok=false as "no estimate", not "estimate is zero".
func loadFetchDurationEstimate(homeDir, kind string) (estimate time.Duration, ok bool) {
	cache := readLiveFetchCache[map[string]fetchDurationSample](fetchDurationsPath(homeDir))
	if cache == nil || cache.Payload == nil {
		return 0, false
	}
	sample, present := cache.Payload[kind]
	if !present || sample.EstimateMS <= 0 {
		return 0, false
	}
	return time.Duration(sample.EstimateMS) * time.Millisecond, true
}

// recordFetchDuration folds one observed fetch duration into kind's
// persisted rolling estimate (EWMA) and writes the cache back. Best-effort:
// a failed lock (a sibling process's write in flight) or a failed write
// (read-only filesystem, missing state dir permissions) silently skips
// persistence rather than interrupting the dashboard -- the splash simply
// keeps falling back to the indeterminate sweep until a write succeeds.
func recordFetchDuration(homeDir, kind string, elapsed time.Duration) {
	if elapsed <= 0 {
		return
	}
	path := fetchDurationsPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}

	unlockInProcess := lockLiveFetchInProcess(path)
	defer unlockInProcess()

	lockFile, ok := lockLiveFetchCache(path)
	if !ok {
		return
	}
	defer unlockLiveFetchCache(lockFile)

	kinds := map[string]fetchDurationSample{}
	if cache := readLiveFetchCache[map[string]fetchDurationSample](path); cache != nil && cache.Payload != nil {
		kinds = cache.Payload
	}

	observedMS := elapsed.Milliseconds()
	next := fetchDurationSample{EstimateMS: observedMS, Samples: 1}
	if prev, existed := kinds[kind]; existed && prev.EstimateMS > 0 {
		blended := fetchDurationEWMAAlpha*float64(observedMS) + (1-fetchDurationEWMAAlpha)*float64(prev.EstimateMS)
		next = fetchDurationSample{EstimateMS: int64(blended), Samples: prev.Samples + 1}
	}
	kinds[kind] = next

	_ = writeLiveFetchCache(path, liveFetchCache[map[string]fetchDurationSample]{
		FetchedAt: time.Now(),
		Payload:   kinds,
	})
}
