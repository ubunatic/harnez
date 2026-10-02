package usage

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// usageTableRow is one quota window of one agent in the plain usage table,
// the normal view mode. Agent-level columns (tokens, freshness, model,
// account) are filled only on an agent's first row.
type usageTableRow struct {
	agent, quota, used, resets string
	tokens, perMin, updated    string
	model, account             string
	stale                      bool
}

// usageTableColumns lists the table columns in display order. The columns
// least needed to read quota state come last, so fit() truncating a narrow
// terminal cuts them first.
var usageTableColumns = []struct {
	title  string
	right  bool
	tokens bool
	value  func(r usageTableRow) string
}{
	{title: "Agent", value: func(r usageTableRow) string { return r.agent }},
	{title: "Quota", value: func(r usageTableRow) string { return r.quota }},
	{title: "Used", right: true, value: func(r usageTableRow) string { return r.used }},
	{title: "Resets", value: func(r usageTableRow) string { return r.resets }},
	{title: "Tokens", right: true, tokens: true, value: func(r usageTableRow) string { return r.tokens }},
	{title: "Tok/min", right: true, tokens: true, value: func(r usageTableRow) string { return r.perMin }},
	{title: "Updated", value: func(r usageTableRow) string { return r.updated }},
	{title: "Model", value: func(r usageTableRow) string { return r.model }},
	{title: "Account", value: func(r usageTableRow) string { return r.account }},
}

// usageTableLines renders agents as an aligned plain-text table. showTokens
// selects the Tokens and Tok/min columns; rates may be nil outside --watch.
func usageTableLines(agents []AgentUsage, rates map[string]agentRate, showTokens bool, now time.Time) []string {
	rows := usageTableRows(agents, rates, now)
	if len(rows) == 0 {
		return nil
	}
	var cols []int
	for i, c := range usageTableColumns {
		if !c.tokens || showTokens {
			cols = append(cols, i)
		}
	}
	widths := make([]int, len(usageTableColumns))
	for _, i := range cols {
		widths[i] = visLen(usageTableColumns[i].title)
		for _, r := range rows {
			widths[i] = max(widths[i], visLen(usageTableColumns[i].value(r)))
		}
	}
	format := func(cell func(i int) string) string {
		parts := make([]string, 0, len(cols))
		for _, i := range cols {
			text := cell(i)
			pad := strings.Repeat(" ", widths[i]-visLen(text))
			if usageTableColumns[i].right {
				text = pad + text
			} else {
				text += pad
			}
			parts = append(parts, text)
		}
		return strings.TrimRight(strings.Join(parts, "  "), " ")
	}
	lines := []string{ansiWrap("bold", format(func(i int) string { return usageTableColumns[i].title }))}
	for _, r := range rows {
		line := format(func(i int) string { return usageTableColumns[i].value(r) })
		if r.stale {
			line = staleValueANSI(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func usageTableRows(agents []AgentUsage, rates map[string]agentRate, now time.Time) []usageTableRow {
	agents = sortedUsageAgents(agents)
	var rows []usageTableRow
	for _, agent := range agents {
		stale := agent.IsValueStale()
		first := usageTableRow{agent: agent.Name, model: agent.ActiveModel, account: agent.Account, stale: stale}
		if agent.PlanTier != "" {
			first.account = strings.TrimPrefix(first.account+" · "+agent.PlanTier, " · ")
		}
		first.tokens, first.perMin = "-", "-"
		if agent.Tokens != nil {
			first.tokens = FormatNumber(agent.Tokens.TotalTokens)
			if rate, ok := rates[agent.AgentID]; ok {
				first.perMin = fmt.Sprintf("%.0f", rate.PerMinute)
			}
		}
		first.updated = "-"
		if !agent.LastRefreshed.IsZero() {
			first.updated = FormatAgo(agent.LastRefreshed)
			if stale {
				first.updated += " · stale"
			}
		}

		windows := usageTableWindows(agent, now)
		if len(windows) == 0 {
			first.quota, first.used, first.resets = usageTableUnavailable(agent), "-", "-"
			rows = append(rows, first)
			continue
		}
		for i, w := range windows {
			row := usageTableRow{agent: agent.Name, stale: stale}
			if i == 0 {
				row = first
			}
			row.quota, row.used, row.resets = w.label, quotaWindowPercent(w.w), compactDurationText(w.w)
			if row.resets == "" {
				row.resets = "-"
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// usageTableWindows lists an agent's quota windows with short table labels:
// "weekly"/"5h" for single-pool agents, "<pool> weekly" for model groups.
func usageTableWindows(agent AgentUsage, now time.Time) []namedWindow {
	var out []namedWindow
	if len(agent.ModelGroups) > 0 {
		groups := slices.Clone(agent.ModelGroups)
		slices.SortStableFunc(groups, func(a, b ModelGroup) int { return strings.Compare(a.Name, b.Name) })
		for _, mg := range groups {
			windows := currentQuotaWindows(mg.Windows, now)
			slices.SortStableFunc(windows, func(a, b QuotaWindow) int {
				return quotaWindowKindOrder(a.Name) - quotaWindowKindOrder(b.Name)
			})
			for _, w := range windows {
				out = append(out, namedWindow{label: modelGroupLabel(mg.Name) + " " + quotaWindowKind(w.Name), w: w})
			}
		}
		return out
	}
	if agent.Weekly != nil {
		out = append(out, namedWindow{label: "weekly", w: *agent.Weekly})
	}
	if agent.Session != nil {
		out = append(out, namedWindow{label: "5h", w: *agent.Session})
	}
	return out
}

func usageTableUnavailable(agent AgentUsage) string {
	switch {
	case agent.QuotaFetchError != "":
		return "unavailable (" + agent.QuotaFetchError + ")"
	case agent.Error != "":
		return "unavailable (" + agent.Error + ")"
	}
	return "-"
}

// quotaWindowKind shortens collector window names such as "Weekly (7-day)"
// or "Five Hour Limit Remaining" to "weekly" or "5h".
func quotaWindowKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "week"):
		return "weekly"
	case strings.Contains(lower, "five hour"), strings.Contains(lower, "5-hour"), strings.Contains(lower, "5 hour"), strings.Contains(lower, "session"):
		return "5h"
	}
	return name
}

// quotaWindowKindOrder sorts weekly windows before 5h windows, matching the
// Weekly-then-Session order of single-pool agents.
func quotaWindowKindOrder(name string) int {
	switch quotaWindowKind(name) {
	case "weekly":
		return 0
	case "5h":
		return 1
	}
	return 2
}
