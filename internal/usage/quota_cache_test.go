package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestQuotaCacheReadWriteRoundTrip checks the atomic write-then-rename helper
// produces a file readLiveFetchCache can parse back unchanged.
func TestQuotaCacheReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := liveFetchCachePath(dir)

	want := liveFetchCache[claudeQuotaPayload]{
		FetchedAt: time.Now().Truncate(time.Second),
		Payload: claudeQuotaPayload{
			Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 42},
			Weekly:  &QuotaWindow{Name: "Weekly (7-day)", UsedPercent: 7},
		},
	}
	if err := writeLiveFetchCache(path, want); err != nil {
		t.Fatalf("writeLiveFetchCache: %v", err)
	}

	// No leftover .tmp file after the rename.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no leftover .tmp file, stat err = %v", err)
	}

	got := readLiveFetchCache[claudeQuotaPayload](path)
	if got == nil {
		t.Fatalf("readLiveFetchCache returned nil")
	}
	if !got.FetchedAt.Equal(want.FetchedAt) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, want.FetchedAt)
	}
	if got.Payload.Session == nil || got.Payload.Session.UsedPercent != 42 {
		t.Errorf("Session = %+v, want UsedPercent 42", got.Payload.Session)
	}
	if got.Payload.Weekly == nil || got.Payload.Weekly.UsedPercent != 7 {
		t.Errorf("Weekly = %+v, want UsedPercent 7", got.Payload.Weekly)
	}
}

func TestTurnQuotaTimeoutIsProviderSpecificAndJSONRemainsBackwardCompatible(t *testing.T) {
	if TurnQuotaTimeoutForProvider("claude") != TurnQuotaTimeout || TurnQuotaTimeoutForProvider("codex") != TurnQuotaTimeout {
		t.Fatal("HTTP providers must retain the 1.8s turn quota timeout")
	}
	if TurnQuotaTimeoutForProvider("agy") <= agyUsageCmdTimeout {
		t.Fatalf("AGY turn quota timeout %s does not cover its %s probe", TurnQuotaTimeoutForProvider("agy"), agyUsageCmdTimeout)
	}

	var old TurnQuotaReading
	if err := json.Unmarshal([]byte(`{"captured_at":"2026-09-24T00:00:00Z","has_cache":true}`), &old); err != nil {
		t.Fatalf("decode old quota reading: %v", err)
	}
	if old.ProbeDurationMS != 0 {
		t.Fatalf("old reading probe duration = %d, want zero", old.ProbeDurationMS)
	}
}

func TestCaptureTurnQuotaIncludesAGYProbeDuration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	prevFn := runAGYUsageCmdFn
	runAGYUsageCmdFn = func(context.Context) ([]byte, error) {
		time.Sleep(25 * time.Millisecond)
		return []byte(agyOKOutput), nil
	}
	defer func() { runAGYUsageCmdFn = prevFn }()

	reading := CaptureTurnQuotaSinceCache(context.Background(), "agy", true, time.Time{}, time.Time{})
	if reading.ProbeDurationMS < 20 {
		t.Fatalf("reading probe duration = %dms, want >= 20ms", reading.ProbeDurationMS)
	}
	encoded, err := json.Marshal(reading)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip TurnQuotaReading
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.ProbeDurationMS != reading.ProbeDurationMS {
		t.Fatalf("serialized probe duration = %dms, want %dms", roundTrip.ProbeDurationMS, reading.ProbeDurationMS)
	}
}

// TestReadQuotaCacheMissing checks a missing cache file is a nil result, not
// an error the caller has to unwrap.
func TestReadQuotaCacheMissing(t *testing.T) {
	dir := t.TempDir()
	if got := readLiveFetchCache[claudeQuotaPayload](liveFetchCachePath(dir)); got != nil {
		t.Errorf("expected nil for missing cache file, got %+v", got)
	}
}

// TestLockQuotaCacheBoundedRetry ensures a writer that can't acquire the
// advisory flock gives up within its retry budget instead of blocking
// indefinitely (issue 033's core requirement: flock doesn't detect a hung
// holder, so a waiter must bound its own wait).
func TestLockQuotaCacheBoundedRetry(t *testing.T) {
	dir := t.TempDir()
	path := liveFetchCachePath(dir)

	// Hold the lock ourselves, simulating a concurrent harnez process mid-write.
	holder, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer holder.Close()
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("flock: %v", err)
	}

	start := time.Now()
	f, ok := lockLiveFetchCache(path)
	elapsed := time.Since(start)

	if ok {
		t.Errorf("expected lock acquisition to fail while held by another fd")
		unlockLiveFetchCache(f)
	}
	maxWait := liveFetchLockDelay*time.Duration(liveFetchLockRetries) + 2*time.Second
	if elapsed > maxWait {
		t.Errorf("lockLiveFetchCache took %v, want bounded well under %v", elapsed, maxWait)
	}
}

// TestLockQuotaCacheSucceedsWhenFree checks the happy path: an uncontended
// lock file is acquired immediately and can be released and re-acquired.
func TestLockQuotaCacheSucceedsWhenFree(t *testing.T) {
	dir := t.TempDir()
	path := liveFetchCachePath(dir)

	f, ok := lockLiveFetchCache(path)
	if !ok {
		t.Fatalf("expected to acquire uncontended lock")
	}
	unlockLiveFetchCache(f)

	f2, ok := lockLiveFetchCache(path)
	if !ok {
		t.Fatalf("expected to re-acquire lock after release")
	}
	unlockLiveFetchCache(f2)
}

// claudeFixtureDir writes the minimal settings/stats/credentials files
// CollectClaude needs to reach the live-quota step, and returns the dir.
func claudeFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{
		"claudeAiOauth": {"accessToken": "mock-token", "subscriptionType": "max"}
	}`), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	return dir
}

// TestCollectClaudeUsesWarmDiskCache checks the core mechanism of issue 033:
// a fresh cache file (within MinWatchInterval) is used directly and the live
// HTTP endpoint is never hit — this is what stops concurrent harnez
// processes from double-polling.
func TestCollectClaudeUsesWarmDiskCache(t *testing.T) {
	dir := claudeFixtureDir(t)

	called := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not be called", http.StatusTeapot)
	}))
	defer mockServer.Close()

	cache := liveFetchCache[claudeQuotaPayload]{
		FetchedAt: time.Now(),
		Payload: claudeQuotaPayload{
			Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 55},
			Weekly:  &QuotaWindow{Name: "Weekly (7-day)", UsedPercent: 11},
		},
	}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), cache); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := CollectClaude(context.Background(), dir, mockServer.Client())

	if called {
		t.Errorf("expected live endpoint not to be called when disk cache is warm")
	}
	if usage.Session == nil || usage.Session.UsedPercent != 55 {
		t.Errorf("Session = %+v, want UsedPercent 55 from cache", usage.Session)
	}
	if usage.Weekly == nil || usage.Weekly.UsedPercent != 11 {
		t.Errorf("Weekly = %+v, want UsedPercent 11 from cache", usage.Weekly)
	}
}

func TestCollectClaudeForcedRefreshBypassesWarmCache(t *testing.T) {
	dir := claudeFixtureDir(t)
	cache := liveFetchCache[claudeQuotaPayload]{FetchedAt: time.Now(), Payload: claudeQuotaPayload{Session: &QuotaWindow{Name: "5h", UsedPercent: 12}}}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), cache); err != nil {
		t.Fatal(err)
	}
	called := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"five_hour":{"utilization":71}}`)), Request: r}, nil
	})}
	u := CollectClaude(ForceQuotaFetch(context.Background()), dir, client)
	if !called {
		t.Fatal("forced refresh reused warm cache without querying the endpoint")
	}
	if u.Session == nil || u.Session.UsedPercent != 71 {
		t.Fatalf("Session=%+v, want fresh 71%%", u.Session)
	}
}

func TestCaptureTurnQuotaSinceReusesCacheRefreshedDuringTurn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"token","subscriptionType":"max"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	turnStarted := time.Now().Add(-time.Second)
	cache := liveFetchCache[claudeQuotaPayload]{FetchedAt: time.Now(), Payload: claudeQuotaPayload{Session: &QuotaWindow{Name: "5h", UsedPercent: 38}}}
	if err := writeLiveFetchCache(liveFetchCachePath(claudeDir), cache); err != nil {
		t.Fatal(err)
	}
	called := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return nil, fmt.Errorf("unexpected quota fetch")
	})}
	oldFactory := turnQuotaHTTPClientFactory
	turnQuotaHTTPClientFactory = func() *http.Client { return client }
	defer func() { turnQuotaHTTPClientFactory = oldFactory }()

	reading := CaptureTurnQuotaSinceCache(context.Background(), "claude", false, turnStarted, turnStarted)
	if called {
		t.Fatal("warm cache refreshed during this turn caused an upstream fetch")
	}
	if len(reading.Windows) == 0 || reading.Windows[0].UsedPercent != 38 {
		t.Fatalf("reading=%+v, want cached 38%% window", reading)
	}
}

func TestCaptureTurnQuotaSinceForcesPreTurnCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"token","subscriptionType":"max"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cache := liveFetchCache[claudeQuotaPayload]{FetchedAt: time.Now().Add(-time.Minute), Payload: claudeQuotaPayload{Session: &QuotaWindow{Name: "5h", UsedPercent: 38}}}
	if err := writeLiveFetchCache(liveFetchCachePath(claudeDir), cache); err != nil {
		t.Fatal(err)
	}
	called := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"five_hour":{"utilization":39}}`)), Request: r}, nil
	})}
	oldFactory := turnQuotaHTTPClientFactory
	turnQuotaHTTPClientFactory = func() *http.Client { return client }
	defer func() { turnQuotaHTTPClientFactory = oldFactory }()

	turnStarted := time.Now().Add(-time.Second)
	reading := CaptureTurnQuotaSinceCache(context.Background(), "claude", false, turnStarted, cache.FetchedAt)
	if !called {
		t.Fatal("pre-turn cache did not trigger a live refresh")
	}
	if len(reading.Windows) == 0 || reading.Windows[0].UsedPercent != 39 {
		t.Fatalf("reading=%+v, want refreshed 39%% window", reading)
	}
}

// TestCollectClaudeFallsBackToStaleCacheOnFetchFailure checks that a live
// fetch failure falls back to the disk cache regardless of its age, with
// windows labeled "(stale)" per issue 032's convention.
func TestCollectClaudeFallsBackToStaleCacheOnFetchFailure(t *testing.T) {
	dir := claudeFixtureDir(t)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer mockServer.Close()

	oldCache := liveFetchCache[claudeQuotaPayload]{
		FetchedAt: time.Now().Add(-time.Hour), // well outside MinWatchInterval
		Payload: claudeQuotaPayload{
			Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 33},
			Weekly:  &QuotaWindow{Name: "Weekly (7-day)", UsedPercent: 9},
		},
	}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), oldCache); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := collectClaudeAgainstURL(t, dir, mockServer)

	if usage.QuotaFetchError == "" {
		t.Errorf("expected QuotaFetchError to be set on HTTP 429")
	}
	if usage.Session == nil || usage.Session.UsedPercent != 33 {
		t.Fatalf("Session = %+v, want stale UsedPercent 33 from cache", usage.Session)
	}
	if usage.Session.Name != "Session (5-hour) (stale)" {
		t.Errorf("Session.Name = %q, want stale-labeled", usage.Session.Name)
	}
	if usage.Weekly == nil || usage.Weekly.Name != "Weekly (7-day) (stale)" {
		t.Errorf("Weekly = %+v, want stale-labeled", usage.Weekly)
	}
}

func TestCollectClaudeRefreshesThroughClaudeCLIAfterUnauthorized(t *testing.T) {
	dir := claudeFixtureDir(t)
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{
		"claudeAiOauth": {"accessToken": "old-token", "subscriptionType": "pro"}
	}`), 0600); err != nil {
		t.Fatalf("write initial credentials: %v", err)
	}

	var mu sync.Mutex
	calls := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		if call == 1 {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer new-token" {
			t.Errorf("retry Authorization = %q, want renewed token", got)
		}
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":12},"seven_day":{"utilization":34}}`))
	}))
	defer mockServer.Close()

	original := runClaudeUsageCmdFn
	runClaudeUsageCmdFn = func(context.Context) ([]byte, error) {
		if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{
			"claudeAiOauth": {"accessToken": "new-token", "subscriptionType": "pro"}
		}`), 0600); err != nil {
			t.Fatalf("write renewed credentials: %v", err)
		}
		return []byte("Current session: 12% used\nCurrent week (all models): 34% used"), nil
	}
	defer func() { runClaudeUsageCmdFn = original }()

	usage := collectClaudeAgainstURL(t, dir, mockServer)
	if usage.QuotaFetchError != "" {
		t.Fatalf("QuotaFetchError = %q, want empty after refresh", usage.QuotaFetchError)
	}
	if usage.Session == nil || usage.Session.UsedPercent != 12 {
		t.Errorf("Session = %+v, want 12%%", usage.Session)
	}
	if usage.Weekly == nil || usage.Weekly.UsedPercent != 34 {
		t.Errorf("Weekly = %+v, want 34%%", usage.Weekly)
	}
	if !strings.Contains(strings.Join(usage.Sources, ","), "after claude -p /usage refresh") {
		t.Errorf("Sources = %v, want CLI refresh source", usage.Sources)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Errorf("HTTP calls = %d, want initial failure plus renewed retry", calls)
	}
}

func TestParseClaudeUsageOutput(t *testing.T) {
	payload := parseClaudeUsageOutput([]byte("You are currently using your subscription\nCurrent session: 0% used\nCurrent week (all models): 66% used · resets Sep 12, 7pm (Europe/Berlin)"))
	if payload.Session == nil || payload.Session.UsedPercent != 0 {
		t.Errorf("Session = %+v, want 0%%", payload.Session)
	}
	if payload.Weekly == nil || payload.Weekly.UsedPercent != 66 {
		t.Errorf("Weekly = %+v, want 66%%", payload.Weekly)
	}
}

// TestCollectClaudeConcurrentCallsDoNotDoubleFetch checks that concurrent
// goroutines calling CollectClaude against a cold cache collapse into
// exactly one live HTTP call via lockLiveFetchInProcess (issue 087's
// in-process serialization added on top of issue 033's cross-process flock).
func TestCollectClaudeConcurrentCallsDoNotDoubleFetch(t *testing.T) {
	dir := claudeFixtureDir(t)

	var mu sync.Mutex
	calls := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":25},"seven_day":{"utilization":10}}`))
	}))
	defer mockServer.Close()

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			collectClaudeAgainstURL(t, dir, mockServer)
		}()
	}
	wg.Wait()

	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 1 {
		t.Errorf("expected exactly one live HTTP call across %d concurrent goroutines, got %d", n, got)
	}
}

// collectClaudeAgainstURL calls CollectClaude with a client whose requests
// are redirected to mockServer, since CollectClaude hardcodes the live
// endpoint host rather than taking it as a parameter.
func collectClaudeAgainstURL(t *testing.T, claudeDir string, mockServer *httptest.Server) AgentUsage {
	t.Helper()
	// A fresh *http.Client per call rather than mockServer.Client() (which
	// lazily initializes and caches a shared client on the server) — tests
	// call this concurrently, and mutating a shared client's Transport
	// field from multiple goroutines is itself a data race independent of
	// anything under test.
	client := &http.Client{Transport: redirectTransport{target: mockServer.URL}}
	return CollectClaude(context.Background(), claudeDir, client)
}

type redirectTransport struct{ target string }

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	targetURL, err := u.Parse(rt.target)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	return http.DefaultTransport.RoundTrip(req)
}
