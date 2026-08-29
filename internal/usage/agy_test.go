package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// agyMockQuotaServer starts an httptest server that answers
// RetrieveUserQuotaSummary requests, counting how many were received, and
// stubs findAGYPortsFn to point CollectAGY's live-fetch gate at it. The
// stub is restored by the returned cleanup.
func agyMockQuotaServer(t *testing.T, handler http.HandlerFunc) (calls *int32, cleanup func()) {
	t.Helper()
	var n int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		handler(w, r)
	}))

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	prevFn := findAGYPortsFn
	findAGYPortsFn = func() []int { return []int{port} }

	return &n, func() {
		findAGYPortsFn = prevFn
		server.Close()
	}
}

func agyOKHandler(w http.ResponseWriter, r *http.Request) {
	resp := AGYQuotaResponse{}
	resp.Response.Groups = []struct {
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Buckets     []struct {
			BucketID          string  `json:"bucketId"`
			DisplayName       string  `json:"displayName"`
			Description       string  `json:"description"`
			Window            string  `json:"window"`
			RemainingFraction float64 `json:"remainingFraction"`
			ResetTime         string  `json:"resetTime"`
		} `json:"buckets"`
	}{
		{
			DisplayName: "Gemini",
			Buckets: []struct {
				BucketID          string  `json:"bucketId"`
				DisplayName       string  `json:"displayName"`
				Description       string  `json:"description"`
				Window            string  `json:"window"`
				RemainingFraction float64 `json:"remainingFraction"`
				ResetTime         string  `json:"resetTime"`
			}{
				{DisplayName: "Daily", RemainingFraction: 0.75},
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// TestCollectAGYUsesWarmDiskCache checks the freshness gate short-circuits
// the live RPC entirely when a recent-enough on-disk reading already exists
// (issue 087, generalizing issue 033's Claude-only mechanism).
func TestCollectAGYUsesWarmDiskCache(t *testing.T) {
	calls, cleanup := agyMockQuotaServer(t, agyOKHandler)
	defer cleanup()

	dir := t.TempDir()
	cache := liveFetchCache[agyQuotaPayload]{
		FetchedAt: time.Now(),
		Payload: agyQuotaPayload{
			ModelGroups: []ModelGroup{{Name: "Cached Group", Windows: []QuotaWindow{{Name: "Daily", UsedPercent: 20}}}},
		},
	}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), cache); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := CollectAGY(context.Background(), dir, http.DefaultClient)

	if atomic.LoadInt32(calls) != 0 {
		t.Errorf("expected live RPC not to be called when disk cache is warm, got %d calls", *calls)
	}
	if len(usage.ModelGroups) != 1 || usage.ModelGroups[0].Name != "Cached Group" {
		t.Errorf("ModelGroups = %+v, want cached group", usage.ModelGroups)
	}
}

// TestCollectAGYExpiredCacheTriggersLiveFetch checks a cache older than
// MinWatchInterval is not used directly — the collector proceeds to the live
// RPC call instead.
func TestCollectAGYExpiredCacheTriggersLiveFetch(t *testing.T) {
	calls, cleanup := agyMockQuotaServer(t, agyOKHandler)
	defer cleanup()

	dir := t.TempDir()
	oldCache := liveFetchCache[agyQuotaPayload]{
		FetchedAt: time.Now().Add(-time.Hour),
		Payload: agyQuotaPayload{
			ModelGroups: []ModelGroup{{Name: "Stale Group"}},
		},
	}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), oldCache); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := CollectAGY(context.Background(), dir, http.DefaultClient)

	if atomic.LoadInt32(calls) != 1 {
		t.Errorf("expected exactly one live RPC call for an expired cache, got %d", *calls)
	}
	if len(usage.ModelGroups) != 1 || usage.ModelGroups[0].Name != "Gemini" {
		t.Errorf("ModelGroups = %+v, want fresh live data", usage.ModelGroups)
	}

	// The refreshed reading should have been persisted for sibling processes.
	got := readLiveFetchCache[agyQuotaPayload](liveFetchCachePath(dir))
	if got == nil || len(got.Payload.ModelGroups) != 1 || got.Payload.ModelGroups[0].Name != "Gemini" {
		t.Errorf("expected refreshed cache written to disk, got %+v", got)
	}
}

// TestCollectAGYConcurrentCallsDoNotDoubleFetch checks that when many
// goroutines call CollectAGY concurrently against a cold cache, the
// in-process serialization added by issue 087's generalized gate
// (lockLiveFetchInProcess) collapses them into exactly one live RPC call:
// the first goroutine to acquire the mutex fetches and writes the cache,
// every other goroutine then finds a warm cache once it is unblocked and
// short-circuits instead of firing its own redundant call.
func TestCollectAGYConcurrentCallsDoNotDoubleFetch(t *testing.T) {
	calls, cleanup := agyMockQuotaServer(t, agyOKHandler)
	defer cleanup()

	dir := t.TempDir()

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			CollectAGY(context.Background(), dir, http.DefaultClient)
		}()
	}
	wg.Wait()

	if got := int(atomic.LoadInt32(calls)); got != 1 {
		t.Errorf("expected exactly one live RPC call across %d concurrent goroutines, got %d", n, got)
	}

	// A refreshed cache should be on disk for the next reader.
	if got := readLiveFetchCache[agyQuotaPayload](liveFetchCachePath(dir)); got == nil {
		t.Errorf("expected a cache file to have been written by one of the concurrent callers")
	}
}
