package usage

import (
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
