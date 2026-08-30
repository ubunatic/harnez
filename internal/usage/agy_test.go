package usage

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// agyStubUsageCmd stubs runAGYUsageCmdFn to return a fixed output/error
// pair instead of shelling out to a real `agy` binary, counting how many
// times it was invoked. The stub is restored by the returned cleanup.
func agyStubUsageCmd(t *testing.T, out []byte, err error) (calls *int32, cleanup func()) {
	t.Helper()
	var n int32
	prevFn := runAGYUsageCmdFn
	runAGYUsageCmdFn = func(ctx context.Context) ([]byte, error) {
		atomic.AddInt32(&n, 1)
		return out, err
	}
	return &n, func() { runAGYUsageCmdFn = prevFn }
}

const agyOKOutput = "Quota:\n" +
	"Gemini Models\tWeekly Limit Remaining\t1%\t2026-08-31T16:27:56Z\n" +
	"Gemini Models\tFive Hour Limit Remaining\t91%\t2026-08-30T19:33:52Z\n" +
	"Claude and GPT models\tWeekly Limit Remaining\t31%\t2026-09-04T10:25:29Z\n" +
	"Claude and GPT models\tFive Hour Limit Remaining\t0%\t2026-08-30T19:47:02Z\n"

// TestParseAGYUsageOutput checks the tab-delimited `/usage` table (issue
// 104's addendum) parses into grouped ModelGroups, remaining-percent
// columns convert to used/remaining pairs, and reset timestamps parse.
func TestParseAGYUsageOutput(t *testing.T) {
	groups := parseAGYUsageOutput([]byte(agyOKOutput))

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(groups), groups)
	}
	if groups[0].Name != "Gemini Models" || len(groups[0].Windows) != 2 {
		t.Fatalf("expected Gemini Models with 2 windows, got %+v", groups[0])
	}
	w := groups[0].Windows[0]
	if w.Name != "Weekly Limit Remaining" || w.RemainingPercent != 1 || w.UsedPercent != 99 {
		t.Errorf("unexpected window: %+v", w)
	}
	if w.ResetAt == nil || !w.ResetAt.Equal(time.Date(2026, 8, 31, 16, 27, 56, 0, time.UTC)) {
		t.Errorf("unexpected ResetAt: %v", w.ResetAt)
	}
	if groups[1].Name != "Claude and GPT models" || len(groups[1].Windows) != 2 {
		t.Fatalf("expected Claude and GPT models with 2 windows, got %+v", groups[1])
	}
}

// TestParseAGYUsageOutputSkipsUnparseableLines checks that lines which
// don't split into the expected columns (e.g. an error message printed
// instead of the usual table) are skipped rather than aborting the whole
// parse or producing bogus zero-value windows.
func TestParseAGYUsageOutputSkipsUnparseableLines(t *testing.T) {
	groups := parseAGYUsageOutput([]byte("Error: Individual quota reached. Resets in 1h54m48s.\n"))
	if len(groups) != 0 {
		t.Errorf("expected no groups from unparseable output, got %+v", groups)
	}
}

// TestCollectAGYUsesWarmDiskCache checks the freshness gate short-circuits
// the exec call entirely when a recent-enough on-disk reading already
// exists (issue 087, generalizing issue 033's Claude-only mechanism).
func TestCollectAGYUsesWarmDiskCache(t *testing.T) {
	calls, cleanup := agyStubUsageCmd(t, []byte(agyOKOutput), nil)
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
		t.Errorf("expected agy exec not to be called when disk cache is warm, got %d calls", *calls)
	}
	if len(usage.ModelGroups) != 1 || usage.ModelGroups[0].Name != "Cached Group" {
		t.Errorf("ModelGroups = %+v, want cached group", usage.ModelGroups)
	}
}

// TestCollectAGYExpiredCacheTriggersLiveFetch checks a cache older than
// MinWatchInterval is not used directly — the collector proceeds to the
// `agy -p "/usage"` exec instead.
func TestCollectAGYExpiredCacheTriggersLiveFetch(t *testing.T) {
	calls, cleanup := agyStubUsageCmd(t, []byte(agyOKOutput), nil)
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
		t.Errorf("expected exactly one agy exec call for an expired cache, got %d", *calls)
	}
	if len(usage.ModelGroups) != 2 || usage.ModelGroups[0].Name != "Gemini Models" {
		t.Errorf("ModelGroups = %+v, want fresh live data", usage.ModelGroups)
	}

	// The refreshed reading should have been persisted for sibling processes.
	got := readLiveFetchCache[agyQuotaPayload](liveFetchCachePath(dir))
	if got == nil || len(got.Payload.ModelGroups) != 2 || got.Payload.ModelGroups[0].Name != "Gemini Models" {
		t.Errorf("expected refreshed cache written to disk, got %+v", got)
	}
}

// TestCollectAGYFailedFetchPreservesDiskCache is the core regression test
// for the "don't delete history data on a failed/empty fetch" requirement:
// when `agy -p "/usage"` errors, the prior on-disk cache must survive
// untouched (not overwritten with an empty payload) and CollectAGY must
// still report the last known data, labeled stale, rather than going blank.
func TestCollectAGYFailedFetchPreservesDiskCache(t *testing.T) {
	_, cleanup := agyStubUsageCmd(t, nil, errors.New("exec: \"agy\": executable file not found in $PATH"))
	defer cleanup()

	dir := t.TempDir()
	cachePath := liveFetchCachePath(dir)
	seeded := liveFetchCache[agyQuotaPayload]{
		FetchedAt: time.Now().Add(-time.Hour),
		Payload: agyQuotaPayload{
			ModelGroups: []ModelGroup{{Name: "Gemini Models", Windows: []QuotaWindow{{Name: "Weekly Limit Remaining", UsedPercent: 30, RemainingPercent: 70}}}},
		},
	}
	if err := writeLiveFetchCache(cachePath, seeded); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := CollectAGY(context.Background(), dir, http.DefaultClient)

	if usage.QuotaFetchError == "" {
		t.Errorf("expected QuotaFetchError to be set on a failed exec")
	}
	if len(usage.ModelGroups) != 1 || usage.ModelGroups[0].Name != "Gemini Models" {
		t.Fatalf("expected prior cached ModelGroups to survive a failed fetch, got %+v", usage.ModelGroups)
	}
	if usage.ModelGroups[0].Windows[0].Name != "Weekly Limit Remaining (stale)" {
		t.Errorf("expected window label marked stale, got %q", usage.ModelGroups[0].Windows[0].Name)
	}

	// The on-disk cache itself must not have been clobbered with an empty
	// payload — it should still hold exactly the seeded data.
	onDisk := readLiveFetchCache[agyQuotaPayload](cachePath)
	if onDisk == nil || len(onDisk.Payload.ModelGroups) != 1 || onDisk.Payload.ModelGroups[0].Name != "Gemini Models" {
		t.Errorf("expected on-disk cache to remain untouched by the failed fetch, got %+v", onDisk)
	}
	if !onDisk.FetchedAt.Equal(seeded.FetchedAt) {
		t.Errorf("expected on-disk FetchedAt to remain the original seed time, got %v want %v", onDisk.FetchedAt, seeded.FetchedAt)
	}
}

// TestCollectAGYEmptyFetchPreservesDiskCache is the same guarantee as
// above, but for the case where the exec call succeeds (no error) yet
// returns no parseable quota lines — e.g. agy printed an unexpected
// message instead of the usual table.
func TestCollectAGYEmptyFetchPreservesDiskCache(t *testing.T) {
	_, cleanup := agyStubUsageCmd(t, []byte("Error: Individual quota reached. Resets in 1h54m48s.\n"), nil)
	defer cleanup()

	dir := t.TempDir()
	cachePath := liveFetchCachePath(dir)
	seeded := liveFetchCache[agyQuotaPayload]{
		FetchedAt: time.Now().Add(-time.Hour),
		Payload: agyQuotaPayload{
			ModelGroups: []ModelGroup{{Name: "Gemini Models", Windows: []QuotaWindow{{Name: "Weekly Limit Remaining", UsedPercent: 30, RemainingPercent: 70}}}},
		},
	}
	if err := writeLiveFetchCache(cachePath, seeded); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := CollectAGY(context.Background(), dir, http.DefaultClient)

	if usage.QuotaFetchError == "" {
		t.Errorf("expected QuotaFetchError to be set when no quota lines parse")
	}
	if len(usage.ModelGroups) != 1 || usage.ModelGroups[0].Name != "Gemini Models" {
		t.Fatalf("expected prior cached ModelGroups to survive an empty fetch, got %+v", usage.ModelGroups)
	}

	onDisk := readLiveFetchCache[agyQuotaPayload](cachePath)
	if onDisk == nil || len(onDisk.Payload.ModelGroups) != 1 {
		t.Errorf("expected on-disk cache to remain untouched by the empty fetch, got %+v", onDisk)
	}
}

// TestCollectAGYFailedFetchNoCacheStillReportsError checks that with no
// prior cache to fall back to, a failed fetch leaves ModelGroups empty
// (nothing to preserve) but still surfaces QuotaFetchError rather than
// silently no-op'ing, per issue 104's acceptance criterion 3.
func TestCollectAGYFailedFetchNoCacheStillReportsError(t *testing.T) {
	_, cleanup := agyStubUsageCmd(t, nil, errors.New("context deadline exceeded"))
	defer cleanup()

	dir := t.TempDir()
	usage := CollectAGY(context.Background(), dir, http.DefaultClient)

	if usage.QuotaFetchError == "" {
		t.Errorf("expected QuotaFetchError to be set")
	}
	if len(usage.ModelGroups) != 0 {
		t.Errorf("expected no ModelGroups with neither a live fetch nor a cache, got %+v", usage.ModelGroups)
	}
}

// TestCollectAGYConcurrentCallsDoNotDoubleFetch checks that when many
// goroutines call CollectAGY concurrently against a cold cache, the
// in-process serialization added by issue 087's generalized gate
// (lockLiveFetchInProcess) collapses them into exactly one exec call: the
// first goroutine to acquire the mutex fetches and writes the cache, every
// other goroutine then finds a warm cache once it is unblocked and
// short-circuits instead of firing its own redundant call.
func TestCollectAGYConcurrentCallsDoNotDoubleFetch(t *testing.T) {
	calls, cleanup := agyStubUsageCmd(t, []byte(agyOKOutput), nil)
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
		t.Errorf("expected exactly one agy exec call across %d concurrent goroutines, got %d", n, got)
	}

	// A refreshed cache should be on disk for the next reader.
	if got := readLiveFetchCache[agyQuotaPayload](liveFetchCachePath(dir)); got == nil {
		t.Errorf("expected a cache file to have been written by one of the concurrent callers")
	}
}
