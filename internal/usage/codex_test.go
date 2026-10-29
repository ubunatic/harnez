package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildCodexQuotaWindowLabelsByDuration(t *testing.T) {
	now := time.Now()
	day := 86400
	for _, tc := range []struct {
		name  string
		pw    CodexRateWindow
		label string
	}{
		{"5 hour", CodexRateWindow{LimitWindowSeconds: 18000}, "5-Hour"},
		{"7 day", CodexRateWindow{LimitWindowSeconds: int64(7 * day)}, "Weekly"},
		{"30 day", CodexRateWindow{LimitWindowSeconds: int64(30 * day)}, "Monthly"},
		{"30 day, last week", CodexRateWindow{LimitWindowSeconds: int64(30 * day), ResetAfterSeconds: int64(3 * day)}, "Monthly"},
		{"unknown length, 29 days left", CodexRateWindow{ResetAt: now.Add(29 * 24 * time.Hour).Unix()}, "Monthly"},
		{"unknown length, 29 days left via reset_after", CodexRateWindow{ResetAfterSeconds: int64(29 * day)}, "Monthly"},
		// Time left alone never proves a shorter class, so no "Weekly" guess.
		{"unknown length, 3 days left", CodexRateWindow{ResetAfterSeconds: int64(3 * day)}, "Quota"},
		{"unknown length, no reset", CodexRateWindow{}, "Quota"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pw := tc.pw
			if got := buildCodexQuotaWindow(&pw, now).Name; got != tc.label {
				t.Errorf("Name = %q, want %q", got, tc.label)
			}
		})
	}
}

// A ChatGPT Free account reports one ~30-day window and no 5-hour window.
func TestCollectCodexFreePlanSingleMonthlyWindow(t *testing.T) {
	dir := codexFixtureDir(t)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plan_type":"free","rate_limit":{"allowed":true,"limit_reached":false,
			"primary_window":{"used_percent":0,"limit_window_seconds":2592000,"reset_after_seconds":2505600},
			"secondary_window":null}}`))
	}))
	defer mockServer.Close()

	got := collectCodexAgainstURL(t, dir, mockServer)
	if !got.Authenticated || got.PlanTier != "Free" {
		t.Fatalf("Authenticated = %v, PlanTier = %q; want authenticated Free", got.Authenticated, got.PlanTier)
	}
	if got.Session != nil {
		t.Errorf("Session = %+v, want nil (no phantom 5-hour window)", got.Session)
	}
	if got.Weekly == nil || got.Weekly.Name != "Monthly" {
		t.Fatalf("Weekly = %+v, want the Monthly window", got.Weekly)
	}

	ctx := context.Background()
	home := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "store.sqlite")
	if err := InitializeUsageStore(ctx, home, dbPath); err != nil {
		t.Fatal(err)
	}
	if err := PersistUsageSummary(ctx, home, dbPath, UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{got}}); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadStoredSummary(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Agents) != 1 || stored.Agents[0].Weekly == nil || stored.Agents[0].Weekly.Name != "Monthly" || stored.Agents[0].Session != nil {
		t.Fatalf("stored summary lost the Monthly window: %+v", stored.Agents)
	}
	_, fromWindows, err := ProviderSnapshotFromStoreAt(ctx, dbPath, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if fromWindows.Weekly == nil || !strings.HasPrefix(fromWindows.Weekly.Name, "Monthly") || fromWindows.Session != nil {
		t.Fatalf("window round-trip: Weekly = %+v, Session = %+v; want Monthly and nil", fromWindows.Weekly, fromWindows.Session)
	}
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
	isolateUsageTestStorage(t)
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

// TestCollectCodexUsesWarmStoreSnapshot checks that a fresh authoritative
// store snapshot short-circuits the live HTTP call.
func TestCollectCodexUsesWarmStoreSnapshot(t *testing.T) {
	dir := codexFixtureDir(t)

	called := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not be called", http.StatusTeapot)
	}))
	defer mockServer.Close()

	seedProviderSnapshot(t, "codex", time.Now(), AgentUsage{Session: &QuotaWindow{Name: "5-Hour", UsedPercent: 60}, Weekly: &QuotaWindow{Name: "Weekly", UsedPercent: 15}})

	usage := collectCodexAgainstURL(t, dir, mockServer)

	if called {
		t.Errorf("expected live endpoint not to be called when store snapshot is warm")
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

// TestCollectCodexParsesNestedTokenFields checks that the current Codex CLI
// auth.json, whose tokens object nests oauth_provenance, still authenticates
// and forwards the account ID (issue 991).
func TestCollectCodexParsesNestedTokenFields(t *testing.T) {
	isolateUsageTestStorage(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{
		"auth_mode": "chatgpt",
		"OPENAI_API_KEY": null,
		"tokens": {
			"id_token": "",
			"access_token": "mock-access-token",
			"refresh_token": "mock-refresh-token",
			"oauth_provenance": {"issuer": "https://auth.openai.com", "client_id": "mock-client"},
			"login_owner_id": "mock-owner",
			"account_id": "mock-account"
		},
		"last_refresh": "2026-10-28T12:00:00Z"
	}`), 0600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	var gotAccount string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccount = r.Header.Get("ChatGPT-Account-ID")
		codexWhamOKHandler(w, r)
	}))
	defer mockServer.Close()

	usage := collectCodexAgainstURL(t, dir, mockServer)

	if !usage.Authenticated || usage.Error != "" {
		t.Fatalf("Authenticated = %v, Error = %q; want authenticated without error", usage.Authenticated, usage.Error)
	}
	if gotAccount != "mock-account" {
		t.Errorf("ChatGPT-Account-ID = %q, want mock-account", gotAccount)
	}
	if usage.Session == nil || usage.Session.UsedPercent != 40 {
		t.Errorf("Session = %+v, want UsedPercent 40 from live fetch", usage.Session)
	}
}

func TestAssignCodexWindowNeutralQuotaNeverDisplacesRealWindows(t *testing.T) {
	neutral := QuotaWindow{Name: "Quota", UsedPercent: 1}
	session := QuotaWindow{Name: "5-Hour", UsedPercent: 2}
	weekly := QuotaWindow{Name: "Weekly", UsedPercent: 3}
	monthly := QuotaWindow{Name: "Monthly", UsedPercent: 4}
	for _, tc := range []struct {
		name        string
		in          []QuotaWindow
		wantSession string
		wantWeekly  string
	}{
		{"lone neutral stays out of session", []QuotaWindow{neutral}, "", "Quota"},
		{"neutral then weekly", []QuotaWindow{neutral, weekly}, "", "Weekly"},
		{"weekly then neutral", []QuotaWindow{weekly, neutral}, "", "Weekly"},
		{"neutral then monthly", []QuotaWindow{neutral, monthly}, "", "Monthly"},
		{"session then neutral", []QuotaWindow{session, neutral}, "5-Hour", "Quota"},
		{"neutral then session", []QuotaWindow{neutral, session}, "5-Hour", "Quota"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var u AgentUsage
			for _, w := range tc.in {
				assignCodexWindow(&u, w)
			}
			gotS, gotW := "", ""
			if u.Session != nil {
				gotS = u.Session.Name
			}
			if u.Weekly != nil {
				gotW = u.Weekly.Name
			}
			if gotS != tc.wantSession || gotW != tc.wantWeekly {
				t.Errorf("Session=%q Weekly=%q, want %q / %q", gotS, gotW, tc.wantSession, tc.wantWeekly)
			}
		})
	}
}
