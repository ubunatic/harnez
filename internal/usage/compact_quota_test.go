package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// compactQuotaProbe stands in for the Claude usage endpoint. It counts upstream
// calls, so no test reaches the real endpoint (issue 1303 M2).
type compactQuotaProbe struct {
	fetches int
	body    string
	err     error
}

func installCompactQuotaProbe(t *testing.T, body string, err error) *compactQuotaProbe {
	t.Helper()
	probe := &compactQuotaProbe{body: body, err: err}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		probe.fetches++
		if probe.err != nil {
			return nil, probe.err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(probe.body)), Request: r}, nil
	})}
	old := turnQuotaHTTPClientFactory
	turnQuotaHTTPClientFactory = func(time.Duration) *http.Client { return client }
	t.Cleanup(func() { turnQuotaHTTPClientFactory = old })
	return probe
}

// compactQuotaHome isolates storage and writes Claude credentials under the
// scratch HOME, returning the Claude directory the compaction sample reads.
func compactQuotaHome(t *testing.T) string {
	t.Helper()
	isolateUsageTestStorage(t)
	dir := filepath.Join(os.Getenv("HOME"), ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"token","subscriptionType":"max"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const compactQuotaBody = `{"five_hour":{"utilization":39}}`

// TestCompactQuotaSpecValues checks that the compaction sample's bounds come from
// spec/usage.yaml (issue 1303 M2), not from Go constants.
func TestCompactQuotaSpecValues(t *testing.T) {
	timeout, maxAge, err := turnQuotaSpecValues()
	if err != nil {
		t.Fatal(err)
	}
	if timeout != 1800*time.Millisecond {
		t.Errorf("turn_quota.timeout = %s, want 1800ms", timeout)
	}
	if maxAge != 5*time.Minute {
		t.Errorf("turn_quota.compact_cache_max_age = %s, want 5m", maxAge)
	}
}

// TestCompactQuotaFreshCacheSkipsFetch: a cache younger than the spec age is read
// and never refetched.
func TestCompactQuotaFreshCacheSkipsFetch(t *testing.T) {
	compactQuotaHome(t)
	probe := installCompactQuotaProbe(t, compactQuotaBody, nil)
	now := time.Now()
	seedProviderSnapshot(t, "claude", now.Add(-time.Minute), AgentUsage{Session: &QuotaWindow{Name: "5h", UsedPercent: 38}})

	got := CaptureCompactQuota(context.Background(), now)
	if probe.fetches != 0 {
		t.Fatalf("fresh cache caused %d upstream fetches, want 0", probe.fetches)
	}
	if got.Source != "compact-cache" || !got.HasCache {
		t.Fatalf("source %q has_cache %v, want compact-cache with a cache", got.Source, got.HasCache)
	}
	if got.CacheAgeMS < 59000 || got.CacheAgeMS > 61000 {
		t.Errorf("cache_age_ms = %d, want about 60000", got.CacheAgeMS)
	}
	if len(got.Windows) == 0 || got.Windows[0].UsedPercent != 38 {
		t.Fatalf("windows = %+v, want the cached 38%% window", got.Windows)
	}
}

// TestCompactQuotaStaleCacheWithFreeLockFetchesOnce: a cache older than the spec
// age is refreshed once, under a free lock.
func TestCompactQuotaStaleCacheWithFreeLockFetchesOnce(t *testing.T) {
	compactQuotaHome(t)
	probe := installCompactQuotaProbe(t, compactQuotaBody, nil)
	now := time.Now()
	seedProviderSnapshot(t, "claude", now.Add(-10*time.Minute), AgentUsage{Session: &QuotaWindow{Name: "5h", UsedPercent: 38}})

	got := CaptureCompactQuota(context.Background(), now)
	if probe.fetches != 1 {
		t.Fatalf("stale cache with a free lock caused %d fetches, want 1", probe.fetches)
	}
	if got.Source != "compact-fetch" || got.Error != "" {
		t.Fatalf("source %q error %q, want compact-fetch without error", got.Source, got.Error)
	}
	if len(got.Windows) == 0 || got.Windows[0].UsedPercent != 39 {
		t.Fatalf("windows = %+v, want the fetched 39%% window", got.Windows)
	}
}

// TestCompactQuotaStaleCacheWithHeldLockSkipsFetch: a stale cache with the lock
// held by another process returns the cached sample with its age and no fetch.
func TestCompactQuotaStaleCacheWithHeldLockSkipsFetch(t *testing.T) {
	dir := compactQuotaHome(t)
	probe := installCompactQuotaProbe(t, compactQuotaBody, nil)
	now := time.Now()
	seedProviderSnapshot(t, "claude", now.Add(-10*time.Minute), AgentUsage{Session: &QuotaWindow{Name: "5h", UsedPercent: 38}})
	lock, ok := lockLiveFetchCacheOnce(liveFetchCachePath(dir))
	if !ok {
		t.Fatal("could not take the test lock")
	}
	defer unlockLiveFetchCache(lock)

	got := CaptureCompactQuota(context.Background(), now)
	if probe.fetches != 0 {
		t.Fatalf("held lock caused %d fetches, want 0", probe.fetches)
	}
	if got.Source != "compact-locked" || got.Error != "" {
		t.Fatalf("source %q error %q, want compact-locked without error", got.Source, got.Error)
	}
	if got.CacheAgeMS < 599000 {
		t.Errorf("cache_age_ms = %d, want about 600000", got.CacheAgeMS)
	}
	if len(got.Windows) == 0 || got.Windows[0].UsedPercent != 38 {
		t.Fatalf("windows = %+v, want the cached 38%% window", got.Windows)
	}
}

// TestCompactQuotaFetchTimeoutRecordsError: a fetch that times out is recorded as
// the reading's error with no windows, and nothing is raised to the caller.
func TestCompactQuotaFetchTimeoutRecordsError(t *testing.T) {
	compactQuotaHome(t)
	probe := installCompactQuotaProbe(t, "", fmt.Errorf("quota fetch: %w", context.DeadlineExceeded))

	got := CaptureCompactQuota(context.Background(), time.Now())
	if probe.fetches != 1 {
		t.Fatalf("fetches = %d, want 1", probe.fetches)
	}
	if got.Error == "" {
		t.Fatal("timed-out fetch recorded no error")
	}
	if len(got.StoreWindows) != 0 || len(got.Windows) != 0 {
		t.Fatalf("timed-out fetch kept windows %+v; a failed sample must not carry quota", got.Windows)
	}
}
