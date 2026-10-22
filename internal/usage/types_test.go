package usage

import (
	"strings"
	"testing"
	"time"
)

func TestQuotaWindowRemainingAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	future := now.Add(2 * time.Hour)
	for _, tc := range []struct {
		name      string
		reset     *time.Time
		stored    time.Duration
		remaining time.Duration
		expired   bool
	}{
		{"future", &future, time.Minute, 2 * time.Hour, false},
		{"past", ptrTime(now.Add(-time.Minute)), time.Hour, 0, true},
		{"legacy", nil, 3 * time.Hour, 3 * time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := QuotaWindow{ResetAt: tc.reset, DurationLeft: tc.stored}
			if got := w.RemainingAt(now); got != tc.remaining {
				t.Errorf("RemainingAt() = %s, want %s", got, tc.remaining)
			}
			if got := w.ExpiredAt(now); got != tc.expired {
				t.Errorf("ExpiredAt() = %v, want %v", got, tc.expired)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestCollectorStatus(t *testing.T) {
	tests := []struct {
		name   string
		agent  AgentUsage
		status string
	}{
		{"live quota", AgentUsage{Session: &QuotaWindow{}}, "live"},
		{"cached", AgentUsage{Sources: []string{"quota-cache.json (cached)"}}, "cache"},
		{"history", AgentUsage{Sources: []string{"usage-history (stale)"}}, "history"},
		{"error", AgentUsage{QuotaFetchError: "timeout"}, "unavailable"},
		{"no data", AgentUsage{Sources: []string{"settings.json"}}, "no-data"},
		{"unknown", AgentUsage{}, "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.agent.CollectorStatus(); got != tc.status {
				t.Errorf("CollectorStatus() = %q, want %q", got, tc.status)
			}
		})
	}
}

// TestAgentUsage_HasUsageData exercises the issue-083 predicate that decides
// whether a renderer should show a box/row for an agent: only once real
// recorded local/remote state was actually found for it, not merely because
// its config directory happens to exist.
func TestAgentUsage_HasUsageData(t *testing.T) {
	tests := []struct {
		name  string
		agent AgentUsage
		want  bool
	}{
		{
			name:  "not installed at all",
			agent: AgentUsage{AgentID: "codex", Installed: false},
			want:  false,
		},
		{
			name:  "installed but empty/unconfigured state dir",
			agent: AgentUsage{AgentID: "agy", Installed: true},
			want:  false,
		},
		{
			name:  "installed and authenticated",
			agent: AgentUsage{AgentID: "claude", Installed: true, Authenticated: true},
			want:  true,
		},
		{
			name:  "installed with local token history but not authenticated",
			agent: AgentUsage{AgentID: "claude", Installed: true, Tokens: &TokenBreakdown{TotalTokens: 42}},
			want:  true,
		},
		{
			name:  "installed with a live quota window recorded",
			agent: AgentUsage{AgentID: "codex", Installed: true, Session: &QuotaWindow{UsedPercent: 10}},
			want:  true,
		},
		{
			name:  "installed with at least one inspected source",
			agent: AgentUsage{AgentID: "agy", Installed: true, Sources: []string{"~/.gemini/antigravity-cli/settings.json"}},
			want:  true,
		},
		{
			name:  "installed with a real error surfaced (e.g. corrupt credentials file)",
			agent: AgentUsage{AgentID: "claude", Installed: true, Error: "invalid credentials format"},
			want:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.agent.HasUsageData(); got != tc.want {
				t.Errorf("HasUsageData() = %v, want %v (agent: %+v)", got, tc.want, tc.agent)
			}
		})
	}
}

func TestAgentUsageStaleReasonAndJSON(t *testing.T) {
	now := time.Now()
	// Fresh: 5m old with multiplier 2 (threshold 10m) is fresh
	fresh := AgentUsage{
		AgentID:       "claude",
		LastRefreshed: now.Add(-5 * time.Minute),
	}
	fresh.PopulateStaleness()
	if fresh.Stale {
		t.Errorf("expected fresh agent at 5m old to not be stale, got reason %q", fresh.StaleReason)
	}
	if fresh.StaleReason != "" {
		t.Errorf("expected empty stale reason for fresh agent, got %q", fresh.StaleReason)
	}

	// Stale by age: 11m old exceeds 10m threshold
	staleAge := AgentUsage{
		AgentID:       "claude",
		LastRefreshed: now.Add(-11 * time.Minute),
	}
	staleAge.PopulateStaleness()
	if !staleAge.Stale {
		t.Errorf("expected agent at 11m old to be stale")
	}
	if !strings.Contains(staleAge.StaleReason, "exceeds stale threshold") {
		t.Errorf("expected age reason, got %q", staleAge.StaleReason)
	}

	// Stale by QuotaFetchError
	staleErr := AgentUsage{
		AgentID:         "agy",
		LastRefreshed:   now.Add(-1 * time.Minute),
		QuotaFetchError: "signal: killed",
	}
	staleErr.PopulateStaleness()
	if !staleErr.Stale {
		t.Errorf("expected agent with QuotaFetchError to be stale")
	}
	if staleErr.StaleReason != "fetch failed: signal: killed" {
		t.Errorf("expected fetch failed reason, got %q", staleErr.StaleReason)
	}

	// Verify RenderJSON populates stale and stale_reason
	summary := UsageSummary{
		Timestamp: now,
		Agents:    []AgentUsage{fresh, staleAge, staleErr},
	}
	jsonOut, err := RenderJSON(summary)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	if !strings.Contains(jsonOut, `"stale": true`) {
		t.Errorf("expected JSON to contain '\"stale\": true', got:\n%s", jsonOut)
	}
	if !strings.Contains(jsonOut, `"stale_reason": "fetch failed: signal: killed"`) {
		t.Errorf("expected JSON to contain '\"stale_reason\": \"fetch failed: signal: killed\"', got:\n%s", jsonOut)
	}
}
