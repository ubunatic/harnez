package usage

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSparkline(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected string
	}{
		{
			name:     "empty slice",
			input:    []float64{},
			expected: "",
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: "",
		},
		{
			name:     "single zero",
			input:    []float64{0},
			expected: " ",
		},
		{
			name:     "all zeros",
			input:    []float64{0, 0, 0, 0},
			expected: "    ",
		},
		{
			name:     "single positive value",
			input:    []float64{100},
			expected: "█",
		},
		{
			name:     "all flat positive",
			input:    []float64{50, 50, 50},
			expected: "███",
		},
		{
			name:     "monotonic increase (8 levels)",
			input:    []float64{0, 10, 20, 30, 40, 50, 60, 70},
			expected: " ▂▃▄▅▆▇█",
		},
		{
			name:     "spiky pattern",
			input:    []float64{10, 100, 10, 100, 0},
			expected: "▂█▂█ ",
		},
		{
			name:     "negative values handled with min baseline",
			input:    []float64{-5, 0, 50, 100},
			expected: " ▂▄█",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderSparkline(tt.input)
			if got != tt.expected {
				t.Errorf("RenderSparkline(%v) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRenderSparklineWidth(t *testing.T) {
	// Baseline v0 scaling: trajectory starting from 100,000 to 110,000
	tokens := []float64{100000, 102000, 105000, 108000, 110000}
	got := RenderSparklineWidth(tokens, 0)
	if got != " ▂▄▆█" {
		t.Errorf("RenderSparklineWidth relative baseline: got %q, want ' ▂▄▆█'", got)
	}

	// Downsampling: 100 items downsampled to 10 characters
	many := make([]float64, 100)
	for i := 0; i < 100; i++ {
		many[i] = float64(i)
	}
	got10 := RenderSparklineWidth(many, 10)
	if len([]rune(got10)) != 10 {
		t.Errorf("expected 10 runes, got %d (%q)", len([]rune(got10)), got10)
	}

	// No downsampling when slice length <= width
	short := []float64{10, 20, 30, 40, 50}
	gotShort := RenderSparklineWidth(short, 10)
	if len([]rune(gotShort)) != 5 {
		t.Errorf("expected 5 runes (no padding or over-sampling), got %d (%q)", len([]rune(gotShort)), gotShort)
	}
}

func TestRenderSparklineInt64(t *testing.T) {
	input := []int64{0, 10, 50, 100}
	expected := " ▂▄█"
	got := RenderSparklineInt64(input)
	if got != expected {
		t.Errorf("RenderSparklineInt64(%v) = %q; want %q", input, got, expected)
	}
}

func TestRenderSparklineInt64Width(t *testing.T) {
	input := []int64{1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000}
	got := RenderSparklineInt64Width(input, 4)
	if len([]rune(got)) != 4 {
		t.Errorf("expected 4 runes, got %d (%q)", len([]rune(got)), got)
	}
}

func TestTimelineSparklines(t *testing.T) {
	t0 := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	entries := []HistoryEntry{
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0,
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 1000,
						},
						ModelTokens: map[string]int64{
							"claude-sonnet-5": 1000,
						},
						Session: &QuotaWindow{
							UsedPercent: 10.0,
						},
					},
				},
			},
		},
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0.Add(10 * time.Minute),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 5000,
						},
						ModelTokens: map[string]int64{
							"claude-sonnet-5": 3000,
							"claude-opus-4":   2000,
						},
						Session: &QuotaWindow{
							UsedPercent: 50.0,
						},
					},
				},
			},
		},
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0.Add(20 * time.Minute),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 10000,
						},
						ModelTokens: map[string]int64{
							"claude-sonnet-5": 6000,
							"claude-opus-4":   4000,
						},
						Session: &QuotaWindow{
							UsedPercent: 90.0,
						},
					},
				},
			},
		},
	}

	rendered := RenderTimelineSparklines(entries)
	if !strings.Contains(rendered, "Claude Code") && !strings.Contains(rendered, "claude") {
		t.Errorf("expected sparklines summary to contain agent claude, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "claude-sonnet-5") {
		t.Errorf("expected model breakdown sparkline for claude-sonnet-5, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "claude-opus-4") {
		t.Errorf("expected model breakdown sparkline for claude-opus-4, got:\n%s", rendered)
	}
	// Verify start → end (+used) and rate calculations:
	// Overall: 1,000 -> 10,000 (+9,000 used) over 20 minutes (1/3 hr) -> 27,000 /hr | 648,000 /day
	if !strings.Contains(rendered, "1,000 →     10,000 (+9,000 used) | 27,000 /hr | 648,000 /day") {
		t.Errorf("expected overall stats with rates, got:\n%s", rendered)
	}
	// claude-sonnet-5: 1,000 -> 6,000 (+5,000 used) over 20 minutes -> 15,000 /hr | 360,000 /day
	if !strings.Contains(rendered, "1,000 →      6,000 (+5,000 used) | 15,000 /hr | 360,000 /day") {
		t.Errorf("expected sonnet stats with rates, got:\n%s", rendered)
	}
	// claude-opus-4: 2,000 -> 4,000 (+2,000 used) over 10 minutes (from t0+10m to t0+20m, 1/6 hr) -> 12,000 /hr | 288,000 /day
	if !strings.Contains(rendered, "2,000 →      4,000 (+2,000 used) | 12,000 /hr | 288,000 /day") {
		t.Errorf("expected opus stats with rates, got:\n%s", rendered)
	}
}

func TestTimelineSparklines_EdgeCases(t *testing.T) {
	t0 := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	// Test 1: Single snapshot (duration 0)
	singleEntry := []HistoryEntry{
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0,
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 5000,
						},
					},
				},
			},
		},
	}

	res1 := RenderTimelineSparklines(singleEntry)
	if !strings.Contains(res1, "5,000 →      5,000 (+0 used) | - /hr | - /day") {
		t.Errorf("expected single entry rate '-' placeholder, got:\n%s", res1)
	}

	// Test 2: Sub-minute duration (e.g. 30 seconds)
	subMinuteEntries := []HistoryEntry{
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0,
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 5000,
						},
					},
				},
			},
		},
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0.Add(30 * time.Second),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 6000,
						},
					},
				},
			},
		},
	}

	res2 := RenderTimelineSparklines(subMinuteEntries)
	if !strings.Contains(res2, "5,000 →      6,000 (+1,000 used) | - /hr | - /day") {
		t.Errorf("expected sub-minute duration to show '-' for rates, got:\n%s", res2)
	}

	// Test 3: Multi-day span (e.g. 2 days / 48 hours)
	multiDayEntries := []HistoryEntry{
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0,
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 100000,
						},
					},
				},
			},
		},
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0.Add(48 * time.Hour),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 340000,
						},
					},
				},
			},
		},
	}

	// 240,000 tokens used over 48 hours -> 5,000 /hr | 120,000 /day
	res3 := RenderTimelineSparklines(multiDayEntries)
	if !strings.Contains(res3, "100,000 →    340,000 (+240,000 used) | 5,000 /hr | 120,000 /day") {
		t.Errorf("expected multi-day rates (5,000/hr, 120,000/day), got:\n%s", res3)
	}

	// Test 4: Zero tokens used over duration
	zeroUsedEntries := []HistoryEntry{
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0,
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 5000,
						},
					},
				},
			},
		},
		{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0.Add(2 * time.Hour),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: 5000,
						},
					},
				},
			},
		},
	}

	res4 := RenderTimelineSparklines(zeroUsedEntries)
	if !strings.Contains(res4, "5,000 →      5,000 (+0 used) | 0 /hr | 0 /day") {
		t.Errorf("expected zero token rate (0 /hr | 0 /day), got:\n%s", res4)
	}
}
