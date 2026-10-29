package usage

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/rograph"
)

// watchBoxSymbol returns the bold display symbol (a superscript digit for
// every box in the numbered scheme) shown next to boxID's title, sourced
// from spec/actions.yaml (issue 132) rather than a hardcoded per-box string.
func watchBoxSymbol(boxID string) string {
	return ansiBold + mustWatchActions().boxTitleSymbol(boxID) + "\x1b[0m"
}

// buildProcessesBox renders a panel showing active agent processes on the system or remote host.
func buildProcessesBox(width int, counts *AgentProcessCount) wbox {
	title := watchBoxSymbol("processes") + " Processes"
	if counts == nil {
		c := CountRunningAgentProcesses()
		counts = &c
	}

	var lines []string
	procWord := "processes"
	if counts.Total() == 1 {
		procWord = "process"
	}
	lines = append(lines, fmt.Sprintf("%d active %s", counts.Total(), procWord))
	lines = append(lines, fmt.Sprintf("claude: %d  agy: %d  codex: %d", counts.Claude, counts.AGY, counts.Codex))

	return wbox{title: title, lines: lines, width: width}
}

func buildAllUsageBox(summary UsageSummary, width int, debugOverlay bool) wbox {
	return buildAllUsageBoxAt(summary, width, debugOverlay, time.Now(), DefaultWatchInterval)
}

// buildAllUsageBoxAt renders the aggregate usage panel using one timestamp for
// every row. The watch redraw captures this timestamp once so each agent's
// gauge advances together on every frame rather than retaining prior output.
func buildAllUsageBoxAt(summary UsageSummary, width int, debugOverlay bool, now time.Time, refreshInterval time.Duration) wbox {
	if debugOverlay {
		refreshInterval = DefaultCollectorInterval()
	}
	lines := allUsageLinesAt(summary, width-4, debugOverlay, now, refreshInterval)
	if len(lines) == 0 {
		lines = []string{ansiDimGrey + "no quota windows available\x1b[0m"}
	}
	return wbox{title: watchBoxSymbol("all_usage") + " All Usage", lines: lines, width: width}
}

// collectorStatusMarker is deliberately short so it remains useful in the
// compact panels and the narrow All Usage table. Unknown legacy snapshots do
// not get a marker because they carry no reliable provenance signal.
func collectorStatusMarker(agent AgentUsage) string {
	status := agent.CollectorStatus()
	if status == "unknown" {
		return ""
	}
	switch status {
	case "live":
		return "●"
	case "history", "cache":
		return "◇"
	default:
		return "!"
	}
}

func collectorStatusLabel(agent AgentUsage, label string) string {
	marker := collectorStatusMarker(agent)
	if marker == "" {
		return label
	}
	if agent.QuotaFetchError != "" {
		return marker + " " + label + " unavailable (" + agent.QuotaFetchError + ")"
	}
	return marker + " " + label
}

func allUsageLines(summary UsageSummary, contentW int, debugOverlay bool) []string {
	return allUsageLinesAt(summary, contentW, debugOverlay, time.Now(), DefaultWatchInterval)
}

func allUsageLinesAt(summary UsageSummary, contentW int, debugOverlay bool, now time.Time, refreshInterval time.Duration) []string {
	type allUsageRow struct {
		label         string
		windows       []QuotaWindow
		lastRefreshed time.Time
		stale         bool
	}
	var rows []allUsageRow
	labelWidth := 0
	for _, agent := range sortedUsageAgents(summary.Agents) {
		if !agent.HasUsageData() {
			continue
		}
		// Snapshot age and fetch failures affect every value; expired or
		// explicitly stale windows affect only the row that contains them.
		stale := agent.IsValueStale()
		if len(agent.ModelGroups) > 0 {
			hasWindow := false
			labelSuffix := ""
			if agent.QuotaFetchError != "" {
				labelSuffix = " unavailable (" + agent.QuotaFetchError + ")"
			} else if agent.Error != "" {
				labelSuffix = " unavailable (" + agent.Error + ")"
			}
			groups := slices.Clone(agent.ModelGroups)
			slices.SortStableFunc(groups, func(a, b ModelGroup) int {
				return strings.Compare(a.Name, b.Name)
			})
			for _, mg := range groups {
				label := modelGroupLabel(mg.Name) + labelSuffix
				if len(mg.Windows) > 0 {
					hasWindow = true
				}
				rowStale := stale
				for _, window := range mg.Windows {
					rowStale = rowStale || quotaWindowIsStale(window, now)
				}
				rows = append(rows, allUsageRow{label: label, windows: currentQuotaWindows(mg.Windows, now), lastRefreshed: agent.LastRefreshed, stale: rowStale})
				if n := visLen(label); n > labelWidth {
					labelWidth = n
				}
			}
			if hasWindow {
				continue
			}
			if marker := collectorStatusMarker(agent); marker != "" {
				label := collectorStatusLabel(agent, agent.Name)
				rows = append(rows, allUsageRow{label: label, lastRefreshed: agent.LastRefreshed, stale: stale})
				if n := visLen(label); n > labelWidth {
					labelWidth = n
				}
			}
			continue
		}

		var wins []QuotaWindow
		if agent.Weekly != nil {
			wins = append(wins, *agent.Weekly)
		}
		if agent.Session != nil {
			wins = append(wins, *agent.Session)
		}
		if len(wins) > 0 {
			label := agent.Name
			rowStale := stale
			for _, window := range wins {
				rowStale = rowStale || quotaWindowIsStale(window, now)
			}
			rows = append(rows, allUsageRow{label: label, windows: wins, lastRefreshed: agent.LastRefreshed, stale: rowStale})
			if n := visLen(label); n > labelWidth {
				labelWidth = n
			}
		} else if marker := collectorStatusMarker(agent); marker != "" {
			// Keep a collector with evidence but no quota windows visible in the
			// aggregate. The marker and name distinguish a failed/empty fetch from
			// an agent that was never discovered, without fabricating quota bars.
			label := collectorStatusLabel(agent, agent.Name)
			rows = append(rows, allUsageRow{label: label, lastRefreshed: agent.LastRefreshed, stale: stale})
			if n := visLen(label); n > labelWidth {
				labelWidth = n
			}
		}
	}

	var lines []string
	midWidth := 10
	for _, row := range rows {
		if len(row.windows) == 0 {
			continue
		}
		duration := compactDurationText(firstAllUsageWindow(row.windows))
		if duration != "" {
			midWidth = max(midWidth, 4+1+visLen(duration))
		}
	}
	for _, row := range rows {
		label := row.label
		if debugOverlay {
			label = freshnessOverlayLabelForInterval(label, row.lastRefreshed, now, refreshInterval)
		}
		line := formatAllUsageTableLineWithMidWidth(label, row.windows, contentW, labelWidth, midWidth, mustIndicators().usageBarPresentation())
		if debugOverlay {
			line = styleTimeGaugeGlyph(line)
		}
		// Issue 107: dim only, no text marker -- the All Usage aggregate is
		// the tightest-budget compact view (formatCompactGroupLineWithLabelWidth's
		// active field-dropping fallback), and dim-grey costs zero visible
		// width since visLen/stripANSI strip it before any layout math.
		if row.stale {
			line = staleValueANSI(line)
		}
		lines = append(lines, line)
	}
	return lines
}

// sortedUsageAgents returns a copy of agents in a stable provider order.
// Store projections may arrive in map order; sorting a copy keeps rendering
// stable without mutating the shared summary.
func sortedUsageAgents(agents []AgentUsage) []AgentUsage {
	providerOrder := func(id string) int {
		switch id {
		case "claude":
			return 0
		case "codex":
			return 1
		case "agy":
			return 2
		default:
			return 3
		}
	}
	agents = slices.Clone(agents)
	slices.SortStableFunc(agents, func(a, b AgentUsage) int {
		if order := providerOrder(a.AgentID) - providerOrder(b.AgentID); order != 0 {
			return order
		}
		return strings.Compare(a.Name, b.Name)
	})
	return agents
}

// currentQuotaWindows returns a copy of windows in which an expired AGY meter
// window reads as reset (0% used) instead of its last stored value.
func currentQuotaWindows(windows []QuotaWindow, now time.Time) []QuotaWindow {
	current := append([]QuotaWindow(nil), windows...)
	for i := range current {
		if current[i].Source == "agy-meter" && current[i].ExpiredAt(now) {
			current[i].UsedPercent = 0
			current[i].RemainingPercent = 100
			current[i].DurationLeft = 0
		}
	}
	return current
}

// modelGroupLabel shortens AGY model-group names for display.
func modelGroupLabel(name string) string {
	switch {
	case strings.EqualFold(name, "Gemini Models"):
		return "Gemini"
	case strings.EqualFold(name, "Claude and GPT models"), strings.EqualFold(name, "Claude and GPT"):
		return "Claude/GPT"
	}
	return name
}

func formatAllUsageLine(label string, windows []QuotaWindow, contentW, labelWidth int) string {
	return formatAllUsageTableLine(label, windows, contentW, labelWidth)
}

func formatAllUsageTableLine(label string, windows []QuotaWindow, contentW, labelWidth int) string {
	return formatAllUsageTableLineWithPresentation(label, windows, contentW, labelWidth, mustIndicators().usageBarPresentation())
}

func formatAllUsageTableLineWithPresentation(label string, windows []QuotaWindow, contentW, labelWidth int, presentation UsageBarPresentation) string {
	midWidth := 10
	if len(windows) > 0 {
		duration := compactDurationText(firstAllUsageWindow(windows))
		if duration != "" {
			midWidth = max(midWidth, 4+1+visLen(duration))
		}
	}
	return formatAllUsageTableLineWithMidWidth(label, windows, contentW, labelWidth, midWidth, presentation)
}

func formatAllUsageTableLineWithMidWidth(label string, windows []QuotaWindow, contentW, labelWidth, midWidth int, presentation UsageBarPresentation) string {
	if len(windows) == 0 {
		return formatCompactGroupLineWithLabelWidth(label, windows, contentW, labelWidth)
	}
	if len(windows) == 1 {
		return formatAllUsageSingleWindowLineWithMidWidth(label, windows[0], contentW, labelWidth, midWidth, presentation)
	}

	var w1, w2 QuotaWindow
	foundWeekly, found5h := false, false
	for _, w := range windows {
		nameLow := strings.ToLower(w.Name)
		if !foundWeekly && (isLongQuotaWindowName(nameLow)) {
			w1 = w
			foundWeekly = true
		} else if !found5h && (strings.Contains(nameLow, "5-hour") || strings.Contains(nameLow, "five hour") || strings.Contains(nameLow, "session")) {
			w2 = w
			found5h = true
		}
	}
	if !foundWeekly || !found5h {
		w1 = windows[0]
		w2 = windows[1]
	}

	d1 := compactDurationText(w1)
	d2 := compactDurationText(w2)

	b1opts := usageBarOptionsWithPresentation(presentation, w1.UsedPercent)
	b1opts.Width = compactBarWidth
	b2opts := usageBarOptionsWithPresentation(presentation, w2.UsedPercent)
	b2opts.Width = compactBarWidth
	b1 := rograph.RenderBar(w1.UsedPercent, b1opts)
	b2 := rograph.RenderBar(w2.UsedPercent, b2opts)
	prefix := rograph.PadLabel(label, labelWidth) + "  "

	midStr := padWatchUsagePercentWithDurationAndPresentation(presentation, w1.UsedPercent, d1, midWidth)
	endStr := watchUsagePercentWithDurationAndPresentation(presentation, w2.UsedPercent, d2)

	line := prefix + b1 + " " + midStr + " " + b2 + " " + endStr
	if visLen(line) <= contentW {
		return line
	}

	// Drop d2 if too long
	endStrNoD2 := watchUsagePercentWithPresentation(presentation, w2.UsedPercent)
	line = prefix + b1 + " " + midStr + " " + b2 + " " + endStrNoD2
	if visLen(line) <= contentW {
		return line
	}

	// Drop d1 as well
	midStrNoD1 := padWatchUsagePercentWithDurationAndPresentation(presentation, w1.UsedPercent, "", 5)
	line = prefix + b1 + " " + midStrNoD1 + " " + b2 + " " + endStrNoD2
	if visLen(line) <= contentW {
		return line
	}

	return prefix + strings.Join([]string{b1, padWatchUsagePercentWithPresentation(presentation, w1.UsedPercent, 4), b2, watchUsagePercentWithPresentation(presentation, w2.UsedPercent)}, " ")
}

func firstAllUsageWindow(windows []QuotaWindow) QuotaWindow {
	for _, window := range windows {
		name := strings.ToLower(window.Name)
		if isLongQuotaWindowName(name) {
			return window
		}
	}
	return windows[0]
}

// blankBarPlaceholder renders an empty bracket the same width as a real
// rograph.RenderBar bar (given watchBarOptions() + Width=4), for a window
// slot that genuinely has no data. It intentionally does NOT call
// rograph.RenderBar(0, ...), which renders a real (if visually empty) 0%
// gauge indistinguishable from a genuinely-empty window — this is a distinct
// "no data" placeholder, not a 0% reading.
func blankBarPlaceholder() string {
	return "[" + strings.Repeat(" ", 4) + "]"
}

// formatAllUsageSingleWindowLine renders a row that has exactly one
// QuotaWindow (e.g. the AGY "Claude/GPT" group when its 5-hour window is
// absent) using the same prefix + bar + midStr column layout as
// formatAllUsageTableLine's two-window rows, but with a blank placeholder
// bracket standing in for the missing second bar. This keeps the row's
// second-bracket column aligned with its box-mates instead of the row simply
// being shorter (issue 172).
func formatAllUsageSingleWindowLine(label string, w QuotaWindow, contentW, labelWidth int) string {
	return formatAllUsageSingleWindowLineWithPresentation(label, w, contentW, labelWidth, mustIndicators().usageBarPresentation())
}

func formatAllUsageSingleWindowLineWithPresentation(label string, w QuotaWindow, contentW, labelWidth int, presentation UsageBarPresentation) string {
	midWidth := 10
	if duration := compactDurationText(w); duration != "" {
		midWidth = max(midWidth, 4+1+visLen(duration))
	}
	return formatAllUsageSingleWindowLineWithMidWidth(label, w, contentW, labelWidth, midWidth, presentation)
}

func formatAllUsageSingleWindowLineWithMidWidth(label string, w QuotaWindow, contentW, labelWidth, midWidth int, presentation UsageBarPresentation) string {
	d1 := compactDurationText(w)

	b1opts := usageBarOptionsWithPresentation(presentation, w.UsedPercent)
	b1opts.Width = compactBarWidth
	b1 := rograph.RenderBar(w.UsedPercent, b1opts)
	b2 := blankBarPlaceholder()
	prefix := rograph.PadLabel(label, labelWidth) + "  "

	midStr := padWatchUsagePercentWithDurationAndPresentation(presentation, w.UsedPercent, d1, midWidth)

	line := prefix + b1 + " " + midStr + " " + b2
	if visLen(line) <= contentW {
		return line
	}

	// Drop d1 if too long, matching the two-window branch's narrow-width
	// fallback pattern.
	midStrNoD1 := padWatchUsagePercentWithDurationAndPresentation(presentation, w.UsedPercent, "", 5)
	line = prefix + b1 + " " + midStrNoD1 + " " + b2
	if visLen(line) <= contentW {
		return line
	}

	return prefix + strings.Join([]string{b1, padWatchUsagePercentWithPresentation(presentation, w.UsedPercent, 4), b2}, " ")
}

func compactDurationText(w QuotaWindow) string {
	remaining := w.RemainingAt(time.Now())
	if remaining <= 0 {
		return ""
	}
	if remaining >= 24*time.Hour && remaining < 48*time.Hour {
		days := int(remaining.Hours()) / 24
		hours := int(remaining.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	return FormatCompactDuration(remaining)
}

// applyStaleQuota fills in a fresh collect result's missing quota windows
// from the previous frame when the fresh fetch failed (issue 032): a single
// transient failure would otherwise blank windows that are very likely still
// accurate a few seconds later. Only fires per-agent when QuotaFetchError is
// set (issue 031) and the corresponding window came back nil/empty; a
// successful fetch always overwrites, so stale data can't accumulate past
// one real refresh. Used only by RunWatch's live loop — one-shot renders
// have no previous frame to fall back to and show the fetch error instead.
func applyStaleQuota(fresh UsageSummary, previous UsageSummary) UsageSummary {
	prevByID := make(map[string]AgentUsage, len(previous.Agents))
	for _, a := range previous.Agents {
		prevByID[a.AgentID] = a
	}
	for i := range fresh.Agents {
		agent := &fresh.Agents[i]
		if agent.QuotaFetchError == "" {
			continue
		}
		prev, ok := prevByID[agent.AgentID]
		if !ok {
			continue
		}
		if agent.Session == nil && prev.Session != nil {
			agent.Session = staleQuotaWindow(prev.Session)
		}
		if agent.Weekly == nil && prev.Weekly != nil {
			agent.Weekly = staleQuotaWindow(prev.Weekly)
		}
		if len(agent.ModelGroups) == 0 && len(prev.ModelGroups) > 0 {
			agent.ModelGroups = staleModelGroups(prev.ModelGroups)
		}
	}
	return fresh
}

// staleQuotaWindow copies a quota window and marks its label stale, so a
// carried-forward panel still reads distinctly from a freshly-refreshed one.
func staleQuotaWindow(w *QuotaWindow) *QuotaWindow {
	cp := *w
	cp.Name = staleLabel(cp.Name)
	return &cp
}

func staleModelGroups(groups []ModelGroup) []ModelGroup {
	out := make([]ModelGroup, len(groups))
	for i, g := range groups {
		ng := g
		ng.Windows = make([]QuotaWindow, len(g.Windows))
		for j, w := range g.Windows {
			w.Name = staleLabel(w.Name)
			ng.Windows[j] = w
		}
		out[i] = ng
	}
	return out
}

func staleLabel(name string) string {
	if strings.HasSuffix(name, " (stale)") {
		return name
	}
	return name + " (stale)"
}

// formatCompactGroupLine formats a model group (or weekly+5h pair) into a single compact line:
// e.g. "Gemini Models    [████] [░░░░]  90% 3d1h   3% 2h17m"
func formatCompactGroupLine(label string, windows []QuotaWindow, contentW int) string {
	return formatCompactGroupLineWithLabelWidth(label, windows, contentW, 10)
}

func formatCompactGroupLineWithLabelWidth(label string, windows []QuotaWindow, contentW, labelWidth int) string {
	if len(windows) == 0 {
		return rograph.PadLabel(label, labelWidth)
	}
	if len(windows) == 1 {
		w := windows[0]
		resetStr := ""
		if w.DurationLeft > 0 {
			resetStr = " " + FormatCompactDuration(w.DurationLeft)
		}
		lbl := rograph.PadLabel(label, labelWidth)
		barOpts := watchUsageBarOptions(w.UsedPercent)
		barOpts.Width = compactBarWidth
		bar := rograph.RenderBar(w.UsedPercent, barOpts)
		line := fmt.Sprintf("%s %s %s%s", lbl, bar, padQuotaWindowPercent(w, 3), resetStr)
		if visLen(line) > contentW {
			line = fmt.Sprintf("%s %s %s", lbl, bar, padQuotaWindowPercent(w, 3))
		}
		return line
	}

	// 2 or more windows: sort/pick Weekly and Short-term (5-Hour) or first 2
	var w1, w2 QuotaWindow
	// Find weekly and 5h windows if possible
	foundWeekly, found5h := false, false
	for _, w := range windows {
		nameLow := strings.ToLower(w.Name)
		if !foundWeekly && (isLongQuotaWindowName(nameLow)) {
			w1 = w
			foundWeekly = true
		} else if !found5h && (strings.Contains(nameLow, "5-hour") || strings.Contains(nameLow, "five hour") || strings.Contains(nameLow, "session")) {
			w2 = w
			found5h = true
		}
	}
	if !foundWeekly || !found5h {
		w1 = windows[0]
		w2 = windows[1]
	}

	d1 := compactDurationText(w1)
	d2 := compactDurationText(w2)

	lbl := rograph.PadLabel(label, labelWidth)
	b1opts := watchUsageBarOptions(w1.UsedPercent)
	b1opts.Width = compactBarWidth
	b2opts := watchUsageBarOptions(w2.UsedPercent)
	b2opts.Width = compactBarWidth
	b1 := rograph.RenderBar(w1.UsedPercent, b1opts)
	b2 := rograph.RenderBar(w2.UsedPercent, b2opts)

	midStr := padQuotaWindowPercentWithDuration(w1, d1, 10)
	endStr := quotaWindowPercentWithDuration(w2, d2)

	// Format: Label [b1] pct1 d1 [b2] pct2 d2 (e.g. Gemini [███░] 91% 2h [░░░░] 0% 3d)
	line := fmt.Sprintf("%s %s %s %s %s", lbl, b1, midStr, b2, endStr)
	if visLen(line) <= contentW {
		return line
	}

	// Drop d2 if too long
	endStrNoD2 := quotaWindowPercent(w2)
	line = fmt.Sprintf("%s %s %s %s %s", lbl, b1, midStr, b2, endStrNoD2)
	if visLen(line) <= contentW {
		return line
	}

	// Drop d1 as well
	midStrNoD1 := padQuotaWindowPercentWithDuration(w1, "", 5)
	line = fmt.Sprintf("%s %s %s %s %s", lbl, b1, midStrNoD1, b2, endStrNoD2)
	if visLen(line) <= contentW {
		return line
	}

	// If still too long in very narrow box, shrink label
	for lw := labelWidth - 1; lw >= 6; lw-- {
		lblShrunk := rograph.PadLabel(label, lw)
		line = fmt.Sprintf("%s %s %s %s %s", lblShrunk, b1, quotaWindowPercent(w1), b2, quotaWindowPercent(w2))
		if visLen(line) <= contentW {
			return line
		}
	}

	// If still too long, shrink label to fit
	return line
}

func quotaWindowPercent(w QuotaWindow) string {
	if w.Source == "agy-meter" {
		return fmt.Sprintf("%.2f%%", w.UsedPercent)
	}
	return watchUsagePercent(w.UsedPercent)
}

func padQuotaWindowPercent(w QuotaWindow, width int) string {
	label := quotaWindowPercent(w)
	return strings.Repeat(" ", max(0, width-visLen(label))) + label
}

func quotaWindowPercentWithDuration(w QuotaWindow, duration string) string {
	label := quotaWindowPercent(w)
	if duration != "" {
		label += " " + duration
	}
	return label
}

func padQuotaWindowPercentWithDuration(w QuotaWindow, duration string, width int) string {
	label := quotaWindowPercentWithDuration(w, duration)
	return label + strings.Repeat(" ", max(0, width-visLen(label)))
}
