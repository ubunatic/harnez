package usage

import "testing"

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
