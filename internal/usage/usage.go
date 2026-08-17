package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CollectAll gathers usage, quotas, and state from all supported agents.
func CollectAll(ctx context.Context, homeDir string, client *http.Client) UsageSummary {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	agyDir := filepath.Join(homeDir, ".gemini", "antigravity-cli")
	codexDir := filepath.Join(homeDir, ".codex")

	claudeUsage := CollectClaude(ctx, claudeDir, client)
	agyUsage := CollectAGY(ctx, agyDir)
	codexUsage := CollectCodex(ctx, codexDir)

	return UsageSummary{
		Timestamp: time.Now(),
		Agents: []AgentUsage{
			claudeUsage,
			agyUsage,
			codexUsage,
		},
	}
}

// RenderJSON serializes the UsageSummary to a formatted JSON string.
func RenderJSON(summary UsageSummary) (string, error) {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal usage summary: %w", err)
	}
	return string(data), nil
}

// RenderText formats the UsageSummary into a clean, human-readable terminal dashboard.
func RenderText(summary UsageSummary) string {
	var sb strings.Builder

	sb.WriteString("Agentic Coding Usage & Quota Monitor\n")
	sb.WriteString(fmt.Sprintf("Snapshot taken at: %s\n\n", summary.Timestamp.Format("2006-01-02 15:04:05 MST")))

	for _, agent := range summary.Agents {
		sb.WriteString(fmt.Sprintf("┌── %s ", agent.Name))
		sb.WriteString(strings.Repeat("─", max(2, 50-len(agent.Name))))
		sb.WriteString("\n")

		if !agent.Installed {
			sb.WriteString("│  Status:       Not installed / Directory not found\n")
			sb.WriteString("└" + strings.Repeat("─", 55) + "\n\n")
			continue
		}

		if !agent.Authenticated {
			sb.WriteString("│  Status:       Installed (Not logged in)\n")
			sb.WriteString("└" + strings.Repeat("─", 55) + "\n\n")
			continue
		}

		// Account & Tier
		var accountStr string
		if agent.Account != "" {
			accountStr = agent.Account
		} else {
			accountStr = "Active Session"
		}
		if agent.PlanTier != "" {
			accountStr += fmt.Sprintf(" (%s Plan)", agent.PlanTier)
		}
		sb.WriteString(fmt.Sprintf("│  Account:      %s\n", accountStr))

		if agent.ActiveModel != "" {
			sb.WriteString(fmt.Sprintf("│  Active Model: %s\n", agent.ActiveModel))
		}

		// Quota Windows
		if agent.Session != nil {
			sb.WriteString("│\n")
			sb.WriteString(fmt.Sprintf("│  %s Limit:\n", agent.Session.Name))
			bar := RenderProgressBar(agent.Session.UsedPercent, 24)
			resetInfo := ""
			if agent.Session.ResetAt != nil {
				localTime := agent.Session.ResetAt.Local().Format("15:04 (MST)")
				if agent.Session.DurationLeft > 0 {
					resetInfo = fmt.Sprintf(" · Resets %s (in %s)", localTime, FormatDuration(agent.Session.DurationLeft))
				} else {
					resetInfo = fmt.Sprintf(" · Resets %s", localTime)
				}
			}
			sb.WriteString(fmt.Sprintf("│    %s %5.1f%% used%s\n", bar, agent.Session.UsedPercent, resetInfo))
		}

		if agent.Weekly != nil {
			if agent.Session == nil {
				sb.WriteString("│\n")
			}
			sb.WriteString(fmt.Sprintf("│  %s Limit:\n", agent.Weekly.Name))
			bar := RenderProgressBar(agent.Weekly.UsedPercent, 24)
			resetInfo := ""
			if agent.Weekly.ResetAt != nil {
				localTime := agent.Weekly.ResetAt.Local().Format("Jan 02, 15:04 (MST)")
				if agent.Weekly.DurationLeft > 0 {
					resetInfo = fmt.Sprintf(" · Resets %s (in %s)", localTime, FormatDuration(agent.Weekly.DurationLeft))
				} else {
					resetInfo = fmt.Sprintf(" · Resets %s", localTime)
				}
			}
			sb.WriteString(fmt.Sprintf("│    %s %5.1f%% used%s\n", bar, agent.Weekly.UsedPercent, resetInfo))
		}

		// Token Breakdown
		if agent.Tokens != nil {
			sb.WriteString("│\n")
			sb.WriteString("│  Token Consumption (Local Totals):\n")
			sb.WriteString(fmt.Sprintf("│    Input:       %14s tokens\n", FormatNumber(agent.Tokens.InputTokens)))
			sb.WriteString(fmt.Sprintf("│    Output:      %14s tokens\n", FormatNumber(agent.Tokens.OutputTokens)))
			sb.WriteString(fmt.Sprintf("│    Cache Read:  %14s tokens\n", FormatNumber(agent.Tokens.CacheReadTokens)))
			sb.WriteString(fmt.Sprintf("│    Cache Write: %14s tokens\n", FormatNumber(agent.Tokens.CacheWriteTokens)))
			sb.WriteString(fmt.Sprintf("│    Total:       %14s tokens\n", FormatNumber(agent.Tokens.TotalTokens)))
		}

		// Additional Details
		if len(agent.Details) > 0 {
			var details []string
			if s, ok := agent.Details["total_sessions"]; ok {
				details = append(details, fmt.Sprintf("%s sessions", s))
			}
			if s, ok := agent.Details["total_conversations"]; ok {
				details = append(details, fmt.Sprintf("%s conversations", s))
			}
			if m, ok := agent.Details["total_messages"]; ok {
				details = append(details, fmt.Sprintf("%s messages", m))
			}
			if r, ok := agent.Details["reasoning_effort"]; ok {
				details = append(details, fmt.Sprintf("reasoning: %s", r))
			}
			if len(details) > 0 {
				sb.WriteString(fmt.Sprintf("│  Activity:     %s\n", strings.Join(details, " · ")))
			}
		}

		sb.WriteString("└" + strings.Repeat("─", 55) + "\n\n")
	}

	return sb.String()
}
