package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAndReadHistory(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	s1 := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 1500,
				},
				ModelTokens: map[string]int64{
					"claude-sonnet-5": 1500,
				},
			},
		},
	}

	if err := AppendHistory(tempDir, s1); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	s2 := UsageSummary{
		Timestamp: now.Add(5 * time.Minute),
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 3000,
				},
				ModelTokens: map[string]int64{
					"claude-sonnet-5": 3000,
				},
			},
		},
	}

	if err := AppendHistory(tempDir, s2); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	entries, err := ReadHistory(tempDir)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	timeline := RenderTimelineText(entries)
	if !strings.Contains(timeline, "Claude Code") {
		t.Errorf("expected timeline to contain 'Claude Code', got:\n%s", timeline)
	}
	if !strings.Contains(timeline, "Usage Trajectory Over Time:") {
		t.Errorf("expected timeline to contain sparklines section, got:\n%s", timeline)
	}
	if !strings.Contains(timeline, "claude-sonnet-5") {
		t.Errorf("expected timeline to show model 'claude-sonnet-5', got:\n%s", timeline)
	}

	jsonOut, err := RenderTimelineJSON(entries)
	if err != nil {
		t.Fatalf("RenderTimelineJSON: %v", err)
	}
	if !strings.Contains(jsonOut, `"claude-sonnet-5": 3000`) {
		t.Errorf("expected JSON to contain model tokens, got:\n%s", jsonOut)
	}
}

func TestRenderTimelineText_Empty(t *testing.T) {
	got := RenderTimelineText(nil)
	if !strings.Contains(got, "No usage history recorded yet") {
		t.Errorf("expected empty message, got %q", got)
	}
}

func TestReadHistory_SkipsCorruptedLines(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test-host.jsonl")
	content := `{"hostname":"test-host","timestamp":"2026-08-23T10:00:00Z","agents":[]}
invalid json line
{"hostname":"test-host","timestamp":"2026-08-23T10:05:00Z","agents":[]}
`
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	entries, err := ReadHistory(tempDir)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after skipping corrupt line, got %d", len(entries))
	}
}

func TestRenderTimelineSparklines_TerminalWidths(t *testing.T) {
	t0 := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	var entries []HistoryEntry
	for i := 0; i < 50; i++ {
		entries = append(entries, HistoryEntry{
			Hostname: "hostA",
			UsageSummary: UsageSummary{
				Timestamp: t0.Add(time.Duration(i) * time.Minute),
				Agents: []AgentUsage{
					{
						AgentID:       "claude",
						Name:          "Claude Code",
						Installed:     true,
						Authenticated: true,
						Tokens: &TokenBreakdown{
							TotalTokens: int64(1000 + i*100),
						},
						ModelTokens: map[string]int64{
							"claude-sonnet-5": int64(1000 + i*100),
						},
					},
				},
			},
		})
	}

	testWidths := []int{40, 80, 90, 120, 160}
	for _, w := range testWidths {
		rendered := RenderTimelineSparklinesWidth(entries, w)
		lines := strings.Split(strings.TrimSpace(rendered), "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "Usage Trajectory") {
				continue
			}
			// Extract sparkline inside brackets [ ... ]
			start := strings.Index(l, "[")
			end := strings.Index(l, "]")
			if start != -1 && end != -1 && end > start {
				spark := l[start+1 : end]
				sparkLen := len([]rune(spark))
				if sparkLen < 5 {
					t.Errorf("width %d: sparkline length %d is less than min bound 5", w, sparkLen)
				}
				if sparkLen > 40 {
					t.Errorf("width %d: sparkline length %d exceeds max bound 40", w, sparkLen)
				}
			}
		}
	}
}

