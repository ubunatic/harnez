package usage

import (
	"bytes"
	"context"
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

func TestHistoryStats(t *testing.T) {
	// Non-existent directory
	fc, tb, te := HistoryStats(filepath.Join(t.TempDir(), "nonexistent"))
	if fc != 0 || tb != 0 || te != 0 {
		t.Errorf("expected (0, 0, 0) for nonexistent dir, got (%d, %d, %d)", fc, tb, te)
	}

	// Empty directory
	emptyDir := t.TempDir()
	fc, tb, te = HistoryStats(emptyDir)
	if fc != 0 || tb != 0 || te != 0 {
		t.Errorf("expected (0, 0, 0) for empty dir, got (%d, %d, %d)", fc, tb, te)
	}

	// Directory with non-.jsonl files and multiple .jsonl files
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a history file"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "host1.jsonl"), []byte("{\"hostname\":\"h1\"}\n{\"hostname\":\"h1\"}\n"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "host2.jsonl"), []byte("{\"hostname\":\"h2\"}\n"), 0600)

	fc, tb, te = HistoryStats(dir)
	if fc != 2 {
		t.Errorf("expected 2 jsonl files, got %d", fc)
	}
	if tb <= 0 {
		t.Errorf("expected totalBytes > 0, got %d", tb)
	}
	if te != 3 {
		t.Errorf("expected 3 entries, got %d", te)
	}
}

func TestHistorySummaryStats(t *testing.T) {
	tempDir := t.TempDir()
	t0 := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	s1 := UsageSummary{
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
			},
			{
				AgentID:       "agy",
				Name:          "Antigravity",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 500,
				},
			},
		},
	}

	s2 := UsageSummary{
		Timestamp: t0.Add(30 * time.Minute),
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 3000,
				},
			},
			{
				AgentID:       "agy",
				Name:          "Antigravity",
				Installed:     true,
				Authenticated: true,
				Tokens: &TokenBreakdown{
					TotalTokens: 1500,
				},
			},
		},
	}

	if err := AppendHistory(tempDir, s1); err != nil {
		t.Fatalf("AppendHistory s1: %v", err)
	}
	if err := AppendHistory(tempDir, s2); err != nil {
		t.Fatalf("AppendHistory s2: %v", err)
	}

	stats, err := HistorySummaryStats(tempDir)
	if err != nil {
		t.Fatalf("HistorySummaryStats: %v", err)
	}

	if stats.FileCount != 1 {
		t.Errorf("expected 1 file, got %d", stats.FileCount)
	}
	if stats.TotalEntries != 2 {
		t.Errorf("expected 2 entries, got %d", stats.TotalEntries)
	}
	if stats.StartTokens != 1500 {
		t.Errorf("expected StartTokens 1500, got %d", stats.StartTokens)
	}
	if stats.EndTokens != 4500 {
		t.Errorf("expected EndTokens 4500, got %d", stats.EndTokens)
	}
	if stats.TotalUsed != 3000 {
		t.Errorf("expected TotalUsed 3000, got %d", stats.TotalUsed)
	}
	if stats.Duration != 30*time.Minute {
		t.Errorf("expected Duration 30m, got %v", stats.Duration)
	}
	// 3000 tokens used over 0.5 hours = 6000/hr, 144000/day
	if stats.RatePerHour != 6000 {
		t.Errorf("expected RatePerHour 6000, got %d", stats.RatePerHour)
	}
	if stats.RatePerDay != 144000 {
		t.Errorf("expected RatePerDay 144000, got %d", stats.RatePerDay)
	}
	if stats.Sparkline == "" {
		t.Errorf("expected non-empty sparkline")
	}
}

func TestFetchRemoteHistory_Validation(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Empty host
	_, err := FetchRemoteHistory(ctx, "", tempDir, nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty host error, got %v", err)
	}

	// Host starting with hyphen (flag injection prevention)
	_, err = FetchRemoteHistory(ctx, "-invalid-flag", tempDir, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid host error, got %v", err)
	}

	// Host with invalid characters
	_, err = FetchRemoteHistory(ctx, "host;rm -rf /", tempDir, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid characters error, got %v", err)
	}

	// Host with spaces
	_, err = FetchRemoteHistory(ctx, "host name", tempDir, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid space error, got %v", err)
	}
}

func TestFetchRemoteHistory_NonExistentHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	var out bytes.Buffer

	// Connect to non-routable / non-listening local port or invalid host
	_, err := FetchRemoteHistory(ctx, "nonexistent-ssh-host-123456", tempDir, &out)
	if err == nil {
		t.Errorf("expected error connecting to non-existent host, got nil")
	}
}

