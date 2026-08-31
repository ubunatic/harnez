package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ubunatic.com/harnez/internal/rograph"
)

// CollectAll gathers usage, quotas, and state from all supported agents. It
// reads each agent's snapshot from the shared collector-daemon cache first
// (see StateDir) and only falls back to a live collect for agents whose
// cached snapshot is missing or older than DefaultCacheStaleness — e.g. the
// daemon (`harnez agent-collector`, issue 082) has never run on this
// machine, or hasn't ticked recently. This keeps existing behavior intact
// when the daemon has never run: every read simply falls back to live
// collection exactly as before issue 082.
func CollectAll(ctx context.Context, homeDir string, client *http.Client) UsageSummary {
	return collectAll(ctx, homeDir, client, true)
}

// CollectAllLive always runs the live collectors, ignoring any cached
// snapshot. This is what `harnez agent-collector` uses on each tick, so the
// daemon never just reads back its own (possibly still-fresh) cache instead
// of actually refreshing it.
func CollectAllLive(ctx context.Context, homeDir string, client *http.Client) UsageSummary {
	return collectAll(ctx, homeDir, client, false)
}

func collectAll(ctx context.Context, homeDir string, client *http.Client, useCache bool) UsageSummary {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	agyDir := filepath.Join(homeDir, ".gemini", "antigravity-cli")
	codexDir := filepath.Join(homeDir, ".codex")

	collectClaude := func() AgentUsage { return CollectClaude(ctx, claudeDir, client) }
	collectAGY := func() AgentUsage { return CollectAGY(ctx, agyDir, client) }
	collectCodex := func() AgentUsage { return CollectCodex(ctx, codexDir, client) }

	var claudeUsage, agyUsage, codexUsage AgentUsage
	if useCache {
		stateDir := StateDir(homeDir)

		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			claudeUsage = cacheOrLive(stateDir, "claude", DefaultCacheStaleness, collectClaude)
		}()
		go func() {
			defer wg.Done()
			agyUsage = cacheOrLive(stateDir, "agy", DefaultCacheStaleness, collectAGY)
		}()
		go func() {
			defer wg.Done()
			codexUsage = cacheOrLive(stateDir, "codex", DefaultCacheStaleness, collectCodex)
		}()
		wg.Wait()

		// The daemon snapshot / live recollect above can still come back
		// with no quota-window data at all (e.g. an intermittently-running
		// agent like AGY hasn't answered in a while, so even its cached
		// snapshot's quota fields were already empty) even though the
		// separately-recorded usage-history log has more recent real
		// quota data from the last time it did answer. Fall back to that
		// as a last resort, purely for display — this never touches the
		// collector-daemon cache itself (issue 086's live-repro follow-up).
		historyDir := HistoryDir(homeDir)
		claudeUsage = fillFromHistoryIfNoQuotaWindows(historyDir, claudeUsage)
		agyUsage = fillFromHistoryIfNoQuotaWindows(historyDir, agyUsage)
		codexUsage = fillFromHistoryIfNoQuotaWindows(historyDir, codexUsage)
	} else {
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			claudeUsage = collectClaude()
		}()
		go func() {
			defer wg.Done()
			agyUsage = collectAGY()
		}()
		go func() {
			defer wg.Done()
			codexUsage = collectCodex()
		}()
		wg.Wait()
	}
	now := time.Now()
	if claudeUsage.LastRefreshed.IsZero() {
		claudeUsage.LastRefreshed = now
	}
	if agyUsage.LastRefreshed.IsZero() {
		agyUsage.LastRefreshed = now
	}
	if codexUsage.LastRefreshed.IsZero() {
		codexUsage.LastRefreshed = now
	}

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

// RenderText formats the UsageSummary into a clean, human-readable terminal
// dashboard.
//
// opts is optional; when its RemoteLoadHost is set (issue 110's
// load.watch_host), a separate remote Load box (same box style the --watch
// grid uses) is appended after the agent boxes, using RemoteLoadSnapshot as
// its data — plain one-shot output has no polling loop of its own, so the
// caller must have already fetched that snapshot via one batch CollectRemote
// call (Decision §2).
func RenderText(summary UsageSummary, opts ...WatchOptions) string {
	opt := firstOpt(opts)
	var sb strings.Builder

	sb.WriteString("Agentic Coding Usage & Quota Monitor\n")
	sb.WriteString(fmt.Sprintf("Snapshot taken at: %s\n\n", summary.Timestamp.Format("2006-01-02 15:04:05 MST")))

	shown := 0
	for _, agent := range summary.Agents {
		if !agent.HasUsageData() {
			continue
		}
		// 7+ day stale agents auto-hide (issue 101) — issue 083's
		// HasUsageData() check alone can't distinguish "genuinely abandoned"
		// from "just refreshed a while ago," since it only looks at whether
		// any field is populated at all, not how old that data is.
		if agent.IsStale(DefaultDisplayStaleness) {
			continue
		}
		shown++
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
				bar := rograph.RenderProgressBar(agent.Session.UsedPercent, rograph.MaxWidth)
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
				bar := rograph.RenderProgressBar(agent.Weekly.UsedPercent, rograph.MaxWidth)
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

			// Live quota fetch failed: surface it explicitly rather than
			// leaving Session/Weekly silently absent (indistinguishable from
			// "this plan has no such window").
			if agent.QuotaFetchError != "" {
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("quota: unavailable (%s)", agent.QuotaFetchError))
			}

			// Model Groups / Multi-Pool Quotas (e.g. Gemini Models vs Claude/GPT Models in AGY)
			for _, mg := range agent.ModelGroups {
				lines = append(lines, "")
				lines = append(lines, strings.ToUpper(mg.Name))
				if mg.Description != "" {
					lines = append(lines, fmt.Sprintf("  %s", mg.Description))
				}
				for _, w := range mg.Windows {
					bar := rograph.RenderProgressBar(w.UsedPercent, rograph.MaxWidth)
					resetInfo := ""
					if w.ResetAt != nil {
						localTime := w.ResetAt.Local().Format("Jan 02, 15:04 (MST)")
						if w.DurationLeft > 0 {
							resetInfo = fmt.Sprintf(" · Resets %s (in %s)", localTime, FormatDuration(w.DurationLeft))
						} else {
							resetInfo = fmt.Sprintf(" · Resets %s", localTime)
						}
					}
					lines = append(lines, fmt.Sprintf("  %-28s %s %5.1f%% used%s", w.Name+":", bar, w.UsedPercent, resetInfo))
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

		// "Last updated" annotation (issue 101): staleness should be visible
		// rather than silently implied, especially once cacheOrLive is
		// serving a last-known snapshot well past the live-recollect window.
		if !agent.LastRefreshed.IsZero() {
			lines = append(lines, fmt.Sprintf("Updated:      %s", FormatAgo(agent.LastRefreshed)))
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
				sb.WriteString(fmt.Sprintf("%s%s\033[0m\n", ansiDimFaint, sl))
			}
		}
		sb.WriteString("\n")
	}

	if shown == 0 {
		sb.WriteString("No supported agent (Claude Code, Codex, AGY) has recorded usage on this\n")
		sb.WriteString("machine yet. Install/configure one and run it, or run `harnez agent-collector\n")
		sb.WriteString("--once` to collect a fresh snapshot, then re-run `harnez usage`.\n")
	}

	if opt.RemoteLoadHost != "" {
		box := buildRemoteLoadBox(remoteLoadBoxWidth, opt.RemoteLoadHost, opt.RemoteLoadSnapshot, opt.RemoteLoadStreaming)
		for _, l := range renderWBox(box) {
			sb.WriteString(l + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// remoteLoadBoxWidth is the fixed panel width RenderText's plain one-shot
// output uses for the remote Load box — plain mode has no live terminal
// column budget to plan against the way the --watch grid does, so this
// picks a width wide enough for the CPU/GPU rows buildLoadBoxLines renders.
const remoteLoadBoxWidth = 48
