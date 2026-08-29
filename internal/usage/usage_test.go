package usage

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSummary(t *testing.T) {
	now := time.Date(2026, 8, 17, 22, 0, 0, 0, time.UTC)
	resetTime := now.Add(4 * time.Hour)

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Account:       "u***@example.com",
				PlanTier:      "Max",
				ActiveModel:   "claude-sonnet-5",
				Session: &QuotaWindow{
					Name:             "Session (5-hour)",
					UsedPercent:      60.0,
					RemainingPercent: 40.0,
					ResetAt:          &resetTime,
					DurationLeft:     4 * time.Hour,
				},
				Tokens: &TokenBreakdown{
					InputTokens:      1000,
					OutputTokens:     2000,
					CacheReadTokens:  3000,
					CacheWriteTokens: 4000,
					TotalTokens:      10000,
				},
				Details: map[string]string{
					"total_sessions": "12",
				},
			},
			{
				AgentID:       "codex",
				Name:          "OpenAI Codex",
				Installed:     false,
				Authenticated: false,
			},
		},
	}

	// Test JSON rendering
	jsonStr, err := RenderJSON(summary)
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	if !strings.Contains(jsonStr, `"agent_id": "claude"`) {
		t.Errorf("expected JSON to contain agent_id claude, got:\n%s", jsonStr)
	}

	// Test Text rendering
	textStr := RenderText(summary)
	if !strings.Contains(textStr, "Claude Code") {
		t.Errorf("expected Text to contain 'Claude Code', got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "60.0% used") {
		t.Errorf("expected Text to contain '60.0%% used', got:\n%s", textStr)
	}
	// codex has no real usage data (Installed: false) — issue 083 says
	// RenderText should self-hide it rather than print an empty/not-installed
	// box for it.
	if strings.Contains(textStr, "OpenAI Codex") {
		t.Errorf("expected Text to omit codex box (no usage data), got:\n%s", textStr)
	}
}

// TestRenderText_StaleWithinSevenDaysStillShown covers issue 101's core
// requirement: an agent whose last known data is older than the 30-minute
// live-recollect window but still well within the 7-day display-hide
// threshold keeps showing its last-known snapshot, annotated with a "last
// updated" timestamp, rather than vanishing or blanking.
func TestRenderText_StaleWithinSevenDaysStillShown(t *testing.T) {
	// IsStale/FormatAgo compare LastRefreshed against actual wall-clock time
	// (not the summary's own Timestamp), so both must be anchored to
	// time.Now() rather than a fixed historical date.
	now := time.Now()
	lastRefreshed := now.Add(-3 * time.Hour) // >> 30min live-recollect window, << 7d display window

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity (AGY)",
				Installed:     true,
				Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: lastRefreshed,
			},
		},
	}

	textStr := RenderText(summary)

	if !strings.Contains(textStr, "Antigravity (AGY)") {
		t.Errorf("expected Text to still show the stale-but-within-7d agent box, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "42.0% used") {
		t.Errorf("expected Text to still show the last-known quota data, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "Updated:") || !strings.Contains(textStr, "3h") {
		t.Errorf("expected Text to annotate the box with a 'last updated ~3h ago' line, got:\n%s", textStr)
	}
}

// TestRenderText_SevenDayStaleAgentHidden covers issue 101's auto-hide gate:
// once an agent's last known data is 7+ days stale, RenderText hides it even
// though HasUsageData() would still report true (a genuinely
// abandoned/uninstalled agent, not one that's merely not running right now).
func TestRenderText_SevenDayStaleAgentHidden(t *testing.T) {
	now := time.Now()
	lastRefreshed := now.Add(-8 * 24 * time.Hour)

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "agy",
				Name:          "Antigravity (AGY)",
				Installed:     true,
				Authenticated: true,
				Session:       &QuotaWindow{Name: "5h", UsedPercent: 42},
				LastRefreshed: lastRefreshed,
			},
		},
	}

	textStr := RenderText(summary)

	if strings.Contains(textStr, "── Antigravity (AGY) ") {
		t.Errorf("expected Text to hide an agent 8 days stale, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "No supported agent") {
		t.Errorf("expected the all-agents-absent fallback message, got:\n%s", textStr)
	}
}

// TestRenderText_AllAgentsAbsent covers issue 083's "don't render a silently
// empty screen" requirement: when no supported agent has any real recorded
// usage, RenderText must say so explicitly instead of printing nothing but
// the header.
func TestRenderText_AllAgentsAbsent(t *testing.T) {
	summary := UsageSummary{
		Timestamp: time.Date(2026, 8, 17, 22, 0, 0, 0, time.UTC),
		Agents: []AgentUsage{
			{AgentID: "claude", Name: "Claude Code", Installed: false},
			{AgentID: "agy", Name: "Antigravity (AGY)", Installed: false},
			{AgentID: "codex", Name: "OpenAI Codex", Installed: false},
		},
	}

	textStr := RenderText(summary)

	// Box titles render as "── <Name> ──"; check for that rather than the
	// bare name, since the explanatory fallback message legitimately mentions
	// the agent names in prose.
	for _, name := range []string{"Claude Code", "Antigravity (AGY)", "OpenAI Codex"} {
		if strings.Contains(textStr, "── "+name+" ") {
			t.Errorf("expected Text to omit %s box (no usage data anywhere), got:\n%s", name, textStr)
		}
	}
	if !strings.Contains(textStr, "No supported agent") {
		t.Errorf("expected Text to explain that no agent has recorded usage, got:\n%s", textStr)
	}
}
