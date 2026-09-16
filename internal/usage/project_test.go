package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectProjectUsage_MockData(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "myrepo")
	_ = os.MkdirAll(repoPath, 0755)

	// Mock Claude dir
	claudeDir := filepath.Join(tmpDir, ".claude")
	projDir := filepath.Join(claudeDir, "projects", "-myrepo")
	_ = os.MkdirAll(projDir, 0755)

	// Write mock claude project jsonl
	jsonlLine := map[string]any{
		"cwd": repoPath,
		"message": map[string]any{
			"model": "claude-sonnet-5",
			"usage": map[string]any{
				"input_tokens":                100,
				"output_tokens":               50,
				"cache_read_input_tokens":     1000,
				"cache_creation_input_tokens": 200,
			},
		},
	}
	data, _ := json.Marshal(jsonlLine)
	_ = os.WriteFile(filepath.Join(projDir, "session1.jsonl"), append(data, '\n'), 0644)

	// Test scanClaudeProjectUsage
	attr := scanClaudeProjectUsage(claudeDir, repoPath)
	if attr == nil || attr.MatchedSessions != 1 {
		t.Fatalf("scanClaudeProjectUsage failed: %+v", attr)
	}
	if attr.Tokens.TotalTokens != 1350 {
		t.Errorf("TotalTokens = %d; want 1350", attr.Tokens.TotalTokens)
	}
	if attr.Tokens.CacheReadTokens != 1000 {
		t.Errorf("CacheReadTokens = %d; want 1000", attr.Tokens.CacheReadTokens)
	}
	if attr.ModelTokens["Sonnet 5"] != 1350 {
		t.Errorf("Sonnet 5 tokens = %d; want 1350", attr.ModelTokens["Sonnet 5"])
	}
}

func TestRenderProjectUsageCard(t *testing.T) {
	res := &ProjectUsageResult{
		RepoDir:        "/home/uwe/projects/harnez",
		RepoName:       "harnez",
		LifetimeTokens: 1_420_000_000,
		TotalTokens: TokenBreakdown{
			InputTokens:      100_000_000,
			CacheReadTokens:  1_300_000_000,
			CacheWriteTokens: 10_000_000,
			OutputTokens:     10_000_000,
			TotalTokens:      1_420_000_000,
		},
		Agents: map[string]*AgentProjectAttribution{
			"claude": {
				AgentID: "claude",
				Name:    "Claude Code",
				Tokens: TokenBreakdown{
					TotalTokens: 1_380_000_000,
				},
				ModelTokens: map[string]int64{
					"Sonnet 5": 938_400_000,
					"Fable 5":  303_600_000,
				},
			},
			"agy": {
				AgentID: "agy",
				Name:    "Antigravity",
				Tokens: TokenBreakdown{
					TotalTokens: 35_200_000,
				},
				ModelTokens: map[string]int64{
					"Gemini 3.7 Flash": 35_200_000,
				},
			},
		},
		CostOfChange: &CostOfChangeMetrics{
			NetCodeLOC:          18400,
			TokensPerLOC:        1690.0,
			ClosedTickets:       337,
			TokensPerTicket:     4_210_000.0,
			CacheReadEfficiency: 97.2,
		},
	}

	card := RenderProjectUsageCard(res)
	if !strings.Contains(card, "Project AI Telemetry: harnez (/home/uwe/projects/harnez)") {
		t.Errorf("card missing header: %s", card)
	}
	if !strings.Contains(card, "Lifetime Attributed Tokens: 1.42B tokens") {
		t.Errorf("card missing lifetime tokens: %s", card)
	}
	if !strings.Contains(card, "Claude Code:") || !strings.Contains(card, "Antigravity:") {
		t.Errorf("card missing agents: %s", card)
	}
	if !strings.Contains(card, "Net Code Lines: +18.4k LOC (~1,690 tokens / LOC)") {
		t.Errorf("card missing net code lines: %s", card)
	}
	if !strings.Contains(card, "Shipped Tickets: 337 closed tickets (~4.2M tokens / ticket)") {
		t.Errorf("card missing shipped tickets: %s", card)
	}
	if !strings.Contains(card, "Prompt Cache Leverage: 97.2% cache read efficiency") {
		t.Errorf("card missing cache efficiency: %s", card)
	}
}
