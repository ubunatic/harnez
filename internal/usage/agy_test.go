package usage

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/agymeter"
)

func writeTestAGYMeterRows(t *testing.T, home string, rows []agymeter.Record) {
	t.Helper()
	path := filepath.Join(home, ".harnez", "agymeter", "usage.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := json.NewEncoder(f).Encode(row); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectAGYPrefersRecentMeterQuotaToUsageCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	geminiDir := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(geminiDir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	reset := now.Add(3 * time.Hour).Format(time.RFC3339)
	zero := 0.0
	rows := []agymeter.Record{
		{Time: now.Add(-2 * time.Minute), Kind: "quota", Bucket: "gemini-weekly", Remaining: floatPtr(0.70), Reset: reset},
		{Time: now.Add(-time.Minute), Kind: "quota", Bucket: "gemini-weekly", Remaining: floatPtr(0.7592765), Reset: reset},
		{Time: now.Add(-time.Minute), Kind: "quota", Bucket: "gemini-5h", Remaining: floatPtr(0.6458407), Reset: reset},
		{Time: now.Add(-time.Minute), Kind: "quota", Bucket: "3p-weekly", Remaining: &zero, Reset: reset},
		{Time: now.Add(-time.Minute), Kind: "quota", Bucket: "3p-5h", Remaining: floatPtr(0.5), Reset: reset},
	}
	writeTestAGYMeterRows(t, home, rows)
	calls, cleanup := agyStubUsageCmd(t, []byte(agyOKOutput), nil)
	defer cleanup()

	got := CollectAGY(context.Background(), geminiDir, http.DefaultClient)
	if atomic.LoadInt32(calls) != 0 {
		t.Fatalf("agy -p /usage called %d times despite fresh meter data", atomic.LoadInt32(calls))
	}
	if len(got.ModelGroups) != 2 || got.ModelGroups[0].Name != "Gemini Models" || got.ModelGroups[1].Name != "Claude and GPT models" {
		t.Fatalf("meter groups = %+v", got.ModelGroups)
	}
	var weekly3P *QuotaWindow
	for i := range got.ModelGroups[1].Windows {
		if got.ModelGroups[1].Windows[i].Name == "Weekly Limit Remaining" {
			weekly3P = &got.ModelGroups[1].Windows[i]
			break
		}
	}
	if weekly3P == nil {
		t.Fatalf("exhausted 3p-weekly bucket missing: %+v", got.ModelGroups[1].Windows)
	}
	if weekly3P.RemainingPercent != 0 || weekly3P.UsedPercent != 100 || weekly3P.Source != "agy-meter" {
		t.Errorf("exhausted 3p-weekly bucket = %+v, want 0%% remaining and 100%% used", *weekly3P)
	}
	w := got.ModelGroups[0].Windows[0]
	if w.Source != "agy-meter" || math.Abs(w.RemainingPercent-75.92765) > 1e-9 || math.Abs(w.UsedPercent-24.07235) > 1e-9 {
		t.Fatalf("meter percentages/source = %+v", w)
	}
	if got.LastRefreshed.IsZero() || time.Since(got.LastRefreshed) > 2*time.Minute {
		t.Errorf("LastRefreshed = %s, want latest meter time", got.LastRefreshed)
	}
	if !strings.Contains(got.Sources[len(got.Sources)-1], ".harnez/agymeter/usage.jsonl") {
		t.Errorf("meter source missing from %v", got.Sources)
	}
	compact := formatCompactGroupLine("Gemini", got.ModelGroups[0].Windows, 100)
	if !strings.Contains(compact, "24.07%") || !strings.Contains(compact, "35.42%") {
		t.Errorf("compact meter percentages lack 2 decimals: %q", compact)
	}
	raw := RenderText(UsageSummary{Agents: []AgentUsage{got}})
	if !strings.Contains(raw, "24.07% used") || !strings.Contains(raw, "Updated:") {
		t.Errorf("raw usage does not show meter percentages to 2 decimals:\n%s", raw)
	}
	if !strings.Contains(raw, "Weekly Limit Remaining:") || !strings.Contains(raw, "100.00% used") {
		t.Errorf("raw usage does not show exhausted weekly bucket:\n%s", raw)
	}
	cached := AgentUsage{AgentID: "agy", ModelGroups: []ModelGroup{{Name: "Old cache"}}}
	overridden, ok := applyRecentAGYMeterQuota(cached, home, now)
	if !ok || len(overridden.ModelGroups) != 2 || overridden.ModelGroups[0].Name != "Gemini Models" {
		t.Errorf("recent meter quota did not override cached quota: %+v, ok=%t", overridden.ModelGroups, ok)
	}
	again, ok := applyRecentAGYMeterQuota(got, home, now)
	if !ok || strings.Count(strings.Join(again.Sources, " "), ".harnez/agymeter/usage.jsonl") != 1 {
		t.Errorf("reapplying meter quota duplicated its source: %v", again.Sources)
	}
}

func TestCollectAGYFallsBackWhenMeterQuotaIsStale(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	geminiDir := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(geminiDir, 0700); err != nil {
		t.Fatal(err)
	}
	writeTestAGYMeterRows(t, home, []agymeter.Record{{Time: time.Now().Add(-DefaultCacheStaleness - time.Minute), Kind: "quota", Bucket: "gemini-weekly", Remaining: floatPtr(0.9)}})
	calls, cleanup := agyStubUsageCmd(t, []byte(agyOKOutput), nil)
	defer cleanup()

	got := CollectAGY(context.Background(), geminiDir, http.DefaultClient)
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("agy -p /usage calls = %d, want 1 fallback", atomic.LoadInt32(calls))
	}
	if len(got.ModelGroups) != 2 || got.ModelGroups[0].Name != "Gemini Models" {
		t.Fatalf("fallback groups = %+v", got.ModelGroups)
	}
}

func floatPtr(value float64) *float64 { return &value }

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

func TestCollectAGYRecordsProbeDurationAndDoesNotBackoffOnTimeout(t *testing.T) {
	dir := t.TempDir()
	prevFn := runAGYUsageCmdFn
	runAGYUsageCmdFn = func(context.Context) ([]byte, error) {
		time.Sleep(25 * time.Millisecond)
		return nil, context.DeadlineExceeded
	}
	defer func() { runAGYUsageCmdFn = prevFn }()

	got := CollectAGY(context.Background(), dir, http.DefaultClient)
	if got.QuotaFetchDurationMS < 20 {
		t.Fatalf("probe duration = %dms, want measured duration >= 20ms", got.QuotaFetchDurationMS)
	}
	if b := readAGYAuthBackoff(agyAuthBackoffPath(dir)); b != nil {
		t.Fatalf("timeout created auth backoff: %+v", b)
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

// TestIsAGYAuthRequired checks the auth/login-required heuristic
// (issue 112) fires on documented reauth-loop phrasing but not on generic
// errors or agy's own normal "trying silent auth" transient log line.
func TestIsAGYAuthRequired(t *testing.T) {
	cases := []struct {
		name string
		out  []byte
		err  error
		want bool
	}{
		{"plain quota table", []byte(agyOKOutput), nil, false},
		{"generic exec error", nil, errors.New("exec: \"agy\": executable file not found in $PATH"), false},
		{"timeout error", nil, context.DeadlineExceeded, false},
		{"unparseable quota line", []byte("Error: Individual quota reached. Resets in 1h54m48s.\n"), nil, false},
		{"login required stdout", []byte("Please log in to continue using Antigravity.\n"), nil, true},
		{"further action required stdout", []byte("Further action is required: please re-authenticate.\n"), nil, true},
		{"interactive login stdout", []byte("Interactive login required — no cached session found.\n"), nil, true},
		{"unauthenticated stdout", []byte("Error: Unauthenticated: token has expired\n"), nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isAGYAuthRequired(c.out, c.err); got != c.want {
				t.Errorf("isAGYAuthRequired(%q, %v) = %v, want %v", c.out, c.err, got, c.want)
			}
		})
	}
}

// TestCollectAGYDetectsAuthRequiredAndBacksOff checks that when
// `agy -p "/usage"` returns auth/login-required output, CollectAGY (a) does
// not treat it as parseable quota data, (b) surfaces a
// reauthentication-specific QuotaFetchError rather than a generic one, (c)
// falls back to the stale on-disk cache like any other failed fetch, and
// (d) persists a backoff marker so a subsequent call within the cooldown
// window skips the `agy` exec entirely (issue 112's mitigation).
func TestCollectAGYDetectsAuthRequiredAndBacksOff(t *testing.T) {
	calls, cleanup := agyStubUsageCmd(t, []byte("Please log in to continue using Antigravity.\n"), nil)
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

	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("expected exactly one agy exec call on first detection, got %d", *calls)
	}
	if !strings.Contains(usage.QuotaFetchError, "reauthentication") {
		t.Errorf("expected QuotaFetchError to mention reauthentication, got %q", usage.QuotaFetchError)
	}
	if len(usage.ModelGroups) != 1 || usage.ModelGroups[0].Name != "Gemini Models" {
		t.Fatalf("expected prior cached ModelGroups to survive, got %+v", usage.ModelGroups)
	}

	// The on-disk quota cache itself must not have been clobbered.
	onDisk := readLiveFetchCache[agyQuotaPayload](cachePath)
	if onDisk == nil || !onDisk.FetchedAt.Equal(seeded.FetchedAt) {
		t.Errorf("expected quota cache to remain untouched, got %+v", onDisk)
	}

	// A backoff marker should now be on disk.
	backoff := readAGYAuthBackoff(agyAuthBackoffPath(dir))
	if backoff == nil || !time.Now().Before(backoff.Until) {
		t.Fatalf("expected an active backoff marker to be written, got %+v", backoff)
	}

	// A second call within the cooldown window must not exec `agy` again,
	// even though the quota cache is still stale.
	usage2 := CollectAGY(context.Background(), dir, http.DefaultClient)
	if atomic.LoadInt32(calls) != 1 {
		t.Errorf("expected no additional agy exec call while backoff is active, got %d total calls", *calls)
	}
	if !strings.Contains(usage2.QuotaFetchError, "backing off") {
		t.Errorf("expected QuotaFetchError to mention the backoff, got %q", usage2.QuotaFetchError)
	}
	if len(usage2.ModelGroups) != 1 || usage2.ModelGroups[0].Name != "Gemini Models" {
		t.Errorf("expected cached ModelGroups to still be reported during backoff, got %+v", usage2.ModelGroups)
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
