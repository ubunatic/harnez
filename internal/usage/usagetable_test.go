package usage

import (
	"strings"
	"testing"
	"time"
)

func usageTableFixture() []AgentUsage {
	return []AgentUsage{
		{
			AgentID: "agy", Name: "Antigravity (AGY)", Installed: true, Authenticated: true,
			Account: "a@example.com", PlanTier: "Consumer", ActiveModel: "Gemini Flash",
			ModelGroups: []ModelGroup{{Name: "Gemini Models", Windows: []QuotaWindow{
				{Name: "Five Hour Limit Remaining", UsedPercent: 7},
				{Name: "Weekly Limit Remaining", UsedPercent: 55},
			}}},
		},
		{
			AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
			PlanTier: "Pro", ActiveModel: "sonnet",
			Weekly:  &QuotaWindow{Name: "Weekly (7-day)", UsedPercent: 92, DurationLeft: 22 * time.Hour},
			Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 10, DurationLeft: 3 * time.Hour},
			Tokens:  &TokenBreakdown{TotalTokens: 1234567},
		},
		{
			AgentID: "codex", Name: "OpenAI Codex", Installed: true, Authenticated: true,
			QuotaFetchError: "no session found",
		},
	}
}

func TestUsageTableOneRowPerQuotaWindow(t *testing.T) {
	lines := usageTableLines(usageTableFixture(), map[string]agentRate{"claude": {PerMinute: 42}}, true, testTime)
	var got []string
	for _, l := range lines {
		got = append(got, strings.Join(strings.Fields(stripANSI(l)), " "))
	}
	want := []string{
		"Agent Quota Used Resets Tokens Tok/min Updated Model Account",
		"Claude Code weekly 92% 22h 1,234,567 42 - sonnet Pro",
		"Claude Code 5h 10% 3h",
		"OpenAI Codex unavailable (no session found) - - - - -",
		"Antigravity (AGY) Gemini weekly 55% - - - - Gemini Flash a@example.com · Consumer",
		"Antigravity (AGY) Gemini 5h 7% -",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("table rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Columns line up: every row's Used cell ends where the header's does.
	header := stripANSI(lines[0])
	usedEnd := strings.Index(header, "Used") + len("Used")
	for _, l := range lines[1:] {
		if c := []rune(stripANSI(l))[usedEnd-1]; c != '%' && c != '-' {
			t.Errorf("Used column misaligned in %q", stripANSI(l))
		}
	}
}

func TestUsageTableHidesTokenColumns(t *testing.T) {
	lines := usageTableLines(usageTableFixture(), nil, false, testTime)
	text := stripANSI(strings.Join(lines, "\n"))
	if strings.Contains(text, "Tokens") || strings.Contains(text, "Tok/min") || strings.Contains(text, "1,234,567") {
		t.Fatalf("token columns shown with showTokens=false:\n%s", text)
	}
}

func TestUsageTableDimsStaleAgent(t *testing.T) {
	agents := []AgentUsage{{
		AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true,
		Sources:       []string{"usage-history (stale)"},
		LastRefreshed: time.Now().Add(-2 * time.Hour),
		Weekly:        &QuotaWindow{Name: "Weekly", UsedPercent: 50},
	}}
	lines := usageTableLines(agents, nil, true, testTime)
	if !strings.Contains(lines[1], ansiDimGrey) {
		t.Fatalf("stale row not dimmed: %q", lines[1])
	}
	if !strings.Contains(stripANSI(lines[1]), "· stale") {
		t.Fatalf("stale row lacks the stale marker: %q", stripANSI(lines[1]))
	}
}

func TestNormalModeShowsTableWithoutBoxes(t *testing.T) {
	summary := UsageSummary{Timestamp: testTime, Agents: usageTableFixture()}
	opt := WatchOptions{Mode: "normal"}
	frame := buildWatchFrame(summary, nil, time.Minute, initialWatchSections(opt), 140, 40, true, opt)
	text := stripANSI(strings.Join(frame.lines, "\n"))
	for _, want := range []string{"Agentic usage", "Agent", "Claude Code", "Gemini weekly", "refresh every"} {
		if !strings.Contains(text, want) {
			t.Errorf("normal frame lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"┌─", "history:", "hidden"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("normal frame contains %q:\n%s", unwanted, text)
		}
	}
}

func TestQuotaWindowKind(t *testing.T) {
	for name, want := range map[string]string{
		"Weekly (7-day) (stale)":    "weekly",
		"Weekly Limit Remaining":    "weekly",
		"Session (5-hour)":          "5h",
		"5-Hour (stale)":            "5h",
		"Five Hour Limit Remaining": "5h",
		"Opus":                      "Opus",
	} {
		if got := quotaWindowKind(name); got != want {
			t.Errorf("quotaWindowKind(%q) = %q, want %q", name, got, want)
		}
	}
}
