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
	"unicode/utf8"
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
	agyUsage := CollectAGY(ctx, agyDir, client)
	codexUsage := CollectCodex(ctx, codexDir, client)

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
		var lines []string

		if !agent.Installed {
			lines = append(lines, "Status:       Not installed / Directory not found")
		} else if !agent.Authenticated {
			lines = append(lines, "Status:       Installed (Not logged in)")
		} else {
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
			lines = append(lines, fmt.Sprintf("Account:      %s", accountStr))

			if agent.ActiveModel != "" {
				lines = append(lines, fmt.Sprintf("Active Model: %s", agent.ActiveModel))
			}

			// Quota Windows (Single Model / Agent Level)
			if agent.Session != nil {
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("%s Limit:", agent.Session.Name))
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
				lines = append(lines, fmt.Sprintf("  %s %5.1f%% used%s", bar, agent.Session.UsedPercent, resetInfo))
			}

			if agent.Weekly != nil {
				if agent.Session == nil {
					lines = append(lines, "")
				}
				lines = append(lines, fmt.Sprintf("%s Limit:", agent.Weekly.Name))
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
				lines = append(lines, fmt.Sprintf("  %s %5.1f%% used%s", bar, agent.Weekly.UsedPercent, resetInfo))
			}

			// Model Groups / Multi-Pool Quotas (e.g. Gemini Models vs Claude/GPT Models in AGY)
			for _, mg := range agent.ModelGroups {
				lines = append(lines, "")
				lines = append(lines, strings.ToUpper(mg.Name))
				if mg.Description != "" {
					lines = append(lines, fmt.Sprintf("  %s", mg.Description))
				}
				for _, w := range mg.Windows {
					bar := RenderProgressBar(w.UsedPercent, 24)
					resetInfo := ""
					if w.ResetAt != nil {
						localTime := w.ResetAt.Local().Format("Jan 02, 15:04 (MST)")
						if w.DurationLeft > 0 {
							resetInfo = fmt.Sprintf(" · Resets %s (in %s)", localTime, FormatDuration(w.DurationLeft))
						} else {
							resetInfo = fmt.Sprintf(" · Resets %s", localTime)
						}
					}
					lines = append(lines, fmt.Sprintf("  %-24s %s %5.1f%% used%s", w.Name+":", bar, w.UsedPercent, resetInfo))
				}
			}

			// Token Breakdown
			if agent.Tokens != nil {
				lines = append(lines, "")
				lines = append(lines, "Token Consumption (Local Totals):")
				lines = append(lines, fmt.Sprintf("  Input:       %14s tokens", FormatNumber(agent.Tokens.InputTokens)))
				lines = append(lines, fmt.Sprintf("  Output:      %14s tokens", FormatNumber(agent.Tokens.OutputTokens)))
				lines = append(lines, fmt.Sprintf("  Cache Read:  %14s tokens", FormatNumber(agent.Tokens.CacheReadTokens)))
				lines = append(lines, fmt.Sprintf("  Cache Write: %14s tokens", FormatNumber(agent.Tokens.CacheWriteTokens)))
				lines = append(lines, fmt.Sprintf("  Total:       %14s tokens", FormatNumber(agent.Tokens.TotalTokens)))
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
					lines = append(lines, fmt.Sprintf("Activity:     %s", strings.Join(details, " · ")))
				}
			}
		}

		// Calculate content width: max visible rune length among lines and title
		// Box content width = maxLineLen + 2 (for 2ch padding after longest line)
		maxLen := utf8.RuneCountInString(agent.Name) + 4
		for _, l := range lines {
			if lLen := utf8.RuneCountInString(l); lLen > maxLen {
				maxLen = lLen
			}
		}
		contentWidth := maxLen + 2

		// Top border: ┌── <Name> ──...──┐
		titleHeader := fmt.Sprintf("── %s ", agent.Name)
		topDashes := contentWidth + 2 - utf8.RuneCountInString(titleHeader)
		if topDashes < 1 {
			topDashes = 1
		}
		sb.WriteString("┌" + titleHeader + strings.Repeat("─", topDashes) + "┐\n")

		// Middle lines: │  <content>   │
		for _, l := range lines {
			if l == "" {
				sb.WriteString("│" + strings.Repeat(" ", contentWidth+2) + "│\n")
			} else {
				padding := contentWidth - utf8.RuneCountInString(l)
				if padding < 0 {
					padding = 0
				}
				sb.WriteString(fmt.Sprintf("│  %s%s│\n", l, strings.Repeat(" ", padding)))
			}
		}

		// Bottom border: └──...──┘
		sb.WriteString("└" + strings.Repeat("─", contentWidth+2) + "┘\n")

		// Dim sources note directly under the box (wrapped to contentWidth+2)
		if len(agent.Sources) > 0 {
			boxWidth := contentWidth + 2
			prefix := " sources: "
			indent := strings.Repeat(" ", len(prefix))

			var sourceLines []string
			curLine := prefix

			for i, src := range agent.Sources {
				item := src
				if i < len(agent.Sources)-1 {
					item += ","
				}

				if curLine == prefix {
					curLine += item
				} else if utf8.RuneCountInString(curLine)+1+utf8.RuneCountInString(item) <= boxWidth {
					curLine += " " + item
				} else {
					sourceLines = append(sourceLines, curLine)
					curLine = indent + item
				}
			}
			if curLine != "" {
				sourceLines = append(sourceLines, curLine)
			}

			for _, sl := range sourceLines {
				sb.WriteString(fmt.Sprintf("\033[2m%s\033[0m\n", sl))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
