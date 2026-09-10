package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestBuildCodexQuotaWindow(t *testing.T) {
	now := time.Now()

	t.Run("short window labeled by hour count", func(t *testing.T) {
		pw := &CodexRateWindow{UsedPercent: 52, LimitWindowSeconds: 5 * 3600, ResetAfterSeconds: 3600}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.Name != "5-Hour" {
			t.Errorf("Name = %q, want 5-Hour", qw.Name)
		}
		if qw.UsedPercent != 52 || qw.RemainingPercent != 48 {
			t.Errorf("UsedPercent/RemainingPercent = %v/%v, want 52/48", qw.UsedPercent, qw.RemainingPercent)
		}
		if qw.ResetAt == nil || qw.DurationLeft != time.Hour {
			t.Errorf("ResetAt/DurationLeft not derived from ResetAfterSeconds: %+v", qw)
		}
	})

	t.Run("day-or-longer window labeled Weekly", func(t *testing.T) {
		pw := &CodexRateWindow{UsedPercent: 30, LimitWindowSeconds: 7 * 86400}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.Name != "Weekly" {
			t.Errorf("Name = %q, want Weekly", qw.Name)
		}
	})

	t.Run("resetAt takes priority over resetAfterSeconds", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Unix()
		pw := &CodexRateWindow{LimitWindowSeconds: 86400, ResetAt: resetAt, ResetAfterSeconds: 999999}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.ResetAt == nil {
			t.Fatalf("expected ResetAt to be set")
		}
		if qw.DurationLeft <= time.Hour || qw.DurationLeft > 2*time.Hour+time.Minute {
			t.Errorf("DurationLeft = %v, want ~2h derived from ResetAt", qw.DurationLeft)
		}
	})
}

func TestCollectCodexTokensAggregatesFinalSessionCounts(t *testing.T) {
	dir := t.TempDir()
	rolloutDir := filepath.Join(dir, "sessions", "2026", "09", "10")
	if err := os.MkdirAll(rolloutDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"session_meta","payload":{"session_id":"s1"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":3,"total_tokens":15}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":20,"cached_input_tokens":4,"output_tokens":5,"total_tokens":29}}}}
malformed
`
	if err := os.WriteFile(filepath.Join(rolloutDir, "rollout-a.jsonl"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, count := collectCodexTokens(dir)
	if count != 1 || got == nil {
		t.Fatalf("collectCodexTokens() = %+v, %d; want one rollout", got, count)
	}
	if got.InputTokens != 20 || got.CacheReadTokens != 4 || got.OutputTokens != 5 || got.TotalTokens != 29 {
		t.Fatalf("token totals = %+v, want final cumulative record", got)
	}
}

func TestCollectCodex_RateLimitWindowAssignment(t *testing.T) {
	now := time.Now()
	resp := CodexWhamUsageResponse{
		RateLimit: &struct {
			Allowed         bool             `json:"allowed"`
			LimitReached    bool             `json:"limit_reached"`
			PrimaryWindow   *CodexRateWindow `json:"primary_window"`
			SecondaryWindow *CodexRateWindow `json:"secondary_window"`
		}{
			PrimaryWindow:   &CodexRateWindow{UsedPercent: 45, LimitWindowSeconds: 5 * 3600},
			SecondaryWindow: &CodexRateWindow{UsedPercent: 12, LimitWindowSeconds: 7 * 86400},
		},
	}

	var usage AgentUsage
	for _, pw := range []*CodexRateWindow{resp.RateLimit.PrimaryWindow, resp.RateLimit.SecondaryWindow} {
		if pw == nil {
			continue
		}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.Name == "Weekly" {
			usage.Weekly = &qw
		} else if usage.Session == nil {
			usage.Session = &qw
		}
	}

	if usage.Session == nil || usage.Session.Name != "5-Hour" || usage.Session.UsedPercent != 45 {
		t.Errorf("Session = %+v, want 5-Hour window at 45%%", usage.Session)
	}
	if usage.Weekly == nil || usage.Weekly.Name != "Weekly" || usage.Weekly.UsedPercent != 12 {
		t.Errorf("Weekly = %+v, want Weekly window at 12%%", usage.Weekly)
	}
}

// codexFixtureDir writes the minimal auth.json CollectCodex needs to reach
// the live-quota step, and returns the dir.
func codexFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{
		"auth_mode": "chatgpt",
		"tokens": {"access_token": "mock-access-token"}
	}`), 0600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	return dir
}

func codexWhamOKHandler(w http.ResponseWriter, r *http.Request) {
	resp := CodexWhamUsageResponse{PlanType: "plus"}
	resp.RateLimit = &struct {
		Allowed         bool             `json:"allowed"`
		LimitReached    bool             `json:"limit_reached"`
		PrimaryWindow   *CodexRateWindow `json:"primary_window"`
		SecondaryWindow *CodexRateWindow `json:"secondary_window"`
	}{
		PrimaryWindow: &CodexRateWindow{UsedPercent: 40, LimitWindowSeconds: 5 * 3600},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// TestCollectCodexUsesWarmDiskCache checks the freshness gate short-circuits
// the live HTTP call entirely when a recent-enough on-disk reading already
// exists (issue 033, generalized to Codex in issue 087).
func TestCollectCodexUsesWarmDiskCache(t *testing.T) {
	dir := codexFixtureDir(t)

	called := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not be called", http.StatusTeapot)
	}))
	defer mockServer.Close()

	cache := liveFetchCache[codexQuotaPayload]{
		FetchedAt: time.Now(),
		Payload: codexQuotaPayload{
			Session: &QuotaWindow{Name: "5-Hour", UsedPercent: 60},
			Weekly:  &QuotaWindow{Name: "Weekly", UsedPercent: 15},
		},
	}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), cache); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := collectCodexAgainstURL(t, dir, mockServer)

	if called {
		t.Errorf("expected live endpoint not to be called when disk cache is warm")
	}
	if usage.Session == nil || usage.Session.UsedPercent != 60 {
		t.Errorf("Session = %+v, want UsedPercent 60 from cache", usage.Session)
	}
	if usage.Weekly == nil || usage.Weekly.UsedPercent != 15 {
		t.Errorf("Weekly = %+v, want UsedPercent 15 from cache", usage.Weekly)
	}
}

// TestCollectCodexExpiredCacheTriggersLiveFetch checks a cache older than
// MinWatchInterval is not used directly — the collector proceeds to the live
// HTTP call instead and persists the refreshed reading.
func TestCollectCodexExpiredCacheTriggersLiveFetch(t *testing.T) {
	dir := codexFixtureDir(t)

	var calls int
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		codexWhamOKHandler(w, r)
	}))
	defer mockServer.Close()

	oldCache := liveFetchCache[codexQuotaPayload]{
		FetchedAt: time.Now().Add(-time.Hour),
		Payload: codexQuotaPayload{
			Session: &QuotaWindow{Name: "5-Hour", UsedPercent: 5},
		},
	}
	if err := writeLiveFetchCache(liveFetchCachePath(dir), oldCache); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	usage := collectCodexAgainstURL(t, dir, mockServer)

	if calls != 1 {
		t.Errorf("expected exactly one live HTTP call for an expired cache, got %d", calls)
	}
	if usage.Session == nil || usage.Session.UsedPercent != 40 {
		t.Errorf("Session = %+v, want fresh UsedPercent 40 from live fetch", usage.Session)
	}

	got := readLiveFetchCache[codexQuotaPayload](liveFetchCachePath(dir))
	if got == nil || got.Payload.Session == nil || got.Payload.Session.UsedPercent != 40 {
		t.Errorf("expected refreshed cache written to disk, got %+v", got)
	}
}

// TestCollectCodexConcurrentCallsDoNotDoubleFetch checks that concurrent
// goroutines calling CollectCodex against a cold cache collapse into exactly
// one live HTTP call via the in-process serialization added in issue 087's
// generalized gate.
func TestCollectCodexConcurrentCallsDoNotDoubleFetch(t *testing.T) {
	dir := codexFixtureDir(t)

	var mu sync.Mutex
	calls := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		codexWhamOKHandler(w, r)
	}))
	defer mockServer.Close()

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			collectCodexAgainstURL(t, dir, mockServer)
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

// collectCodexAgainstURL calls CollectCodex with a client whose requests are
// redirected to mockServer, since CollectCodex hardcodes the live endpoint
// host rather than taking it as a parameter.
func collectCodexAgainstURL(t *testing.T, codexDir string, mockServer *httptest.Server) AgentUsage {
	t.Helper()
	// A fresh *http.Client per call rather than mockServer.Client() (shared
	// and lazily initialized on the server) — this helper is called
	// concurrently by TestCollectCodexConcurrentCallsDoNotDoubleFetch.
	client := &http.Client{Transport: redirectTransport{target: mockServer.URL}}
	return CollectCodex(context.Background(), codexDir, client)
}
