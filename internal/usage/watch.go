package usage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

// DefaultWatchInterval is the default refresh cadence for `harnez usage --watch`.
const DefaultWatchInterval = 60 * time.Second

// MinWatchInterval is the fastest refresh cadence allowed via --interval. It
// exists to stop `--watch` from hammering live quota APIs (Anthropic, AGY,
// Codex) with requests far more often than the data actually changes.
const MinWatchInterval = 30 * time.Second

var sparkRunes = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

const rateHistoryLen = 12
const minBoxWidth = 34
const maxTotalWidth = 100

// boxGap is the number of blank columns combineRow puts between side-by-side
// panels. Layout math must account for it: forgetting the gutter is what made
// a 2-column row 101 cells wide in a 100-column terminal, wrapping every single
// box line onto a second physical row.
const boxGap = 1

// safetyMargin is the number of columns we refuse to use at the right edge.
// Writing into the terminal's very last cell leaves VTE-style terminals in the
// "deferred wrap" state, and any single miscounted glyph (the box-drawing runes,
// `·` and `…` all have East-Asian *Ambiguous* width and may render two cells
// wide) then pushes the line onto the next row. Staying one column short makes
// the layout robust against both.
const safetyMargin = 1

type agentRate struct {
	PerMinute float64
	Spark     string
}

// rateTracker keeps a short in-memory history of token totals per agent so the
// watch view can show a tokens/min trend without persisting anything to disk.
type rateTracker struct {
	mu      sync.Mutex
	lastAt  time.Time
	lastTot map[string]int64
	history map[string][]float64
}

func newRateTracker() *rateTracker {
	return &rateTracker{
		lastTot: make(map[string]int64),
		history: make(map[string][]float64),
	}
}

func (t *rateTracker) update(summary UsageSummary) map[string]agentRate {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string]agentRate)
	now := summary.Timestamp
	elapsedMin := 0.0
	if !t.lastAt.IsZero() {
		elapsedMin = now.Sub(t.lastAt).Minutes()
	}

	for _, agent := range summary.Agents {
		if agent.Tokens == nil {
			continue
		}
		total := agent.Tokens.TotalTokens
		rate := 0.0
		havePrev := false
		if prev, ok := t.lastTot[agent.AgentID]; ok && elapsedMin > 0 && total >= prev {
			rate = float64(total-prev) / elapsedMin
			havePrev = true
		}
		t.lastTot[agent.AgentID] = total

		hist := t.history[agent.AgentID]
		if havePrev {
			hist = append(hist, rate)
			if len(hist) > rateHistoryLen {
				hist = hist[len(hist)-rateHistoryLen:]
			}
			t.history[agent.AgentID] = hist
		}

		result[agent.AgentID] = agentRate{PerMinute: rate, Spark: renderSparkline(hist)}
	}

	t.lastAt = now
	return result
}

func renderSparkline(values []float64) string {
	return RenderSparkline(values)
}

// stripANSI removes ANSI escape sequences so visible width can be measured.
func stripANSI(s string) string {
	var sb strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func visLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func truncateVisible(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}
	if visLen(s) <= maxWidth {
		return s
	}
	var sb strings.Builder
	inEsc := false
	count := 0
	target := maxWidth - 3
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			sb.WriteRune(r)
			continue
		}
		if inEsc {
			sb.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if count >= target {
			break
		}
		sb.WriteRune(r)
		count++
	}
	sb.WriteString("...\x1b[0m")
	return sb.String()
}

// wbox is a single bordered panel in the compact btop-style watch grid.
type wbox struct {
	title string
	lines []string
	width int
}

func renderWBox(b wbox) []string {
	var res []string

	titleVis := visLen(b.title) + 4
	rem := b.width - titleVis - 1
	if rem < 1 {
		rem = 1
	}
	res = append(res, "┌─ "+b.title+" "+strings.Repeat("─", rem)+"┐")

	contentW := b.width - 4
	if contentW < 10 {
		contentW = 10
	}
	for _, l := range b.lines {
		if visLen(l) > contentW {
			l = truncateVisible(l, contentW)
		}
		pad := contentW - visLen(l)
		if pad < 0 {
			pad = 0
		}
		res = append(res, "│ "+l+strings.Repeat(" ", pad)+" │")
	}

	res = append(res, "└"+strings.Repeat("─", b.width-2)+"┘")
	return res
}

// combineRow lays rendered boxes out side by side, padding shorter boxes to
// the tallest one in the row.
func combineRow(cols [][]string) []string {
	maxLines := 0
	widths := make([]int, len(cols))
	for i, c := range cols {
		if len(c) > maxLines {
			maxLines = len(c)
		}
		if len(c) > 0 {
			widths[i] = visLen(c[0])
		}
	}
	res := make([]string, maxLines)
	for i := 0; i < maxLines; i++ {
		var parts []string
		for ci, c := range cols {
			if i < len(c) {
				parts = append(parts, c[i])
			} else {
				parts = append(parts, strings.Repeat(" ", widths[ci]))
			}
		}
		res[i] = strings.Join(parts, strings.Repeat(" ", boxGap))
	}
	return res
}

// Floors on the geometry we lay panels out for. Below these, box borders and
// truncated content stop being legible, so we clamp rather than trust a
// nonsensical reading.
const minTerminalWidth = 30
const minTerminalHeight = 6

// Fallbacks used only when every real size probe fails (no tty at all).
const defaultTerminalWidth = 90
const defaultTerminalHeight = 24

// ttySize queries f's window size directly via TIOCGWINSZ. This is more
// reliable than shelling out to `stty -F /dev/tty size`: it reads the actual
// fd we're writing to instead of a separate /dev/tty open, which can report a
// stale or unrelated size (or fail outright) under some pty wrappers.
func ttySize(f *os.File) (cols, rows int, ok bool) {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return 0, 0, false
	}
	return int(ws.Col), int(ws.Row), true
}

// terminalSize resolves the usable geometry for the watch grid, trying out's
// own fd first (most reliable), then os.Stdout, then a /dev/tty stty query,
// then $COLUMNS/$LINES, before giving up on fixed defaults.
//
// Rows matter as much as columns: the redraw loop must know how tall the
// viewport is so a frame can never overflow it. An overflowing frame scrolls
// the terminal, and once it has scrolled, the `\x1b[H` that starts the next
// redraw no longer points at the row the program thinks it does.
func terminalSize(out io.Writer) (cols, rows int) {
	clamp := func(c, r int) (int, int) {
		if c < minTerminalWidth {
			c = minTerminalWidth
		}
		if r < minTerminalHeight {
			r = minTerminalHeight
		}
		return c, r
	}

	if f, ok := out.(*os.File); ok {
		if c, r, ok := ttySize(f); ok {
			return clamp(c, r)
		}
	}
	if c, r, ok := ttySize(os.Stdout); ok {
		return clamp(c, r)
	}

	if raw, err := exec.Command("stty", "-F", "/dev/tty", "size").Output(); err == nil {
		parts := strings.Fields(string(raw))
		if len(parts) >= 2 {
			r, errR := strconv.Atoi(parts[0])
			c, errC := strconv.Atoi(parts[1])
			if errR == nil && errC == nil && c > 0 && r > 0 {
				return clamp(c, r)
			}
		}
	}

	cols, rows = defaultTerminalWidth, defaultTerminalHeight
	if v, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && v > 0 {
		cols = v
	}
	if v, err := strconv.Atoi(os.Getenv("LINES")); err == nil && v > 0 {
		rows = v
	}
	return clamp(cols, rows)
}

// screenFrame is one fully laid-out frame: lines already truncated to the
// terminal's width and count already capped to its height. Because it can
// never exceed the viewport, painting it is position-independent and cannot
// scroll the terminal.
type screenFrame struct {
	lines []string
	cols  int
	rows  int
}

// paint writes the frame at the top-left of the (alternate) screen.
//
// Every line is followed by an explicit erase-to-end-of-line. That is the part
// the previous implementation missed: `\x1b[H` + content + `\x1b[J` only clears
// *below* the new content, so any line shorter than the one it replaced kept
// the old line's tail, and blank lines (a bare "\n") cleared nothing at all.
// Shrinking the frame — e.g. toggling three panels down to one — therefore left
// debris from the larger frame interleaved with the new box.
//
// No newline is emitted after the final line, so even a frame exactly as tall
// as the terminal cannot push the viewport down by one row.
func (f screenFrame) paint(out io.Writer) {
	var buf bytes.Buffer
	buf.WriteString("\x1b[H")
	for i, line := range f.lines {
		if i > 0 {
			buf.WriteString("\r\n")
		}
		buf.WriteString(line)
		buf.WriteString("\x1b[0m\x1b[K")
	}
	buf.WriteString("\x1b[J")
	_, _ = out.Write(buf.Bytes())
}

// fit truncates lines to cols and caps them at rows, so the frame provably fits.
func fit(lines []string, cols, rows int) screenFrame {
	if len(lines) > rows {
		lines = lines[:rows]
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if visLen(l) > cols {
			l = truncateVisible(l, cols)
		}
		out[i] = l
	}
	return screenFrame{lines: out, cols: cols, rows: rows}
}

type namedWindow struct {
	label string
	w     QuotaWindow
}

// watchSections controls which panels the `--watch` grid currently shows.
// Toggled interactively via keypress; see RunWatch. The key for each panel
// (shown in its own title bar, btop-style) is fixed here.
type watchSections struct {
	Claude    bool
	AGY       bool
	Codex     bool
	History   bool
	Processes bool
	Load      bool
	Tokens    bool
}

func defaultWatchSections() watchSections {
	return watchSections{Claude: true, AGY: true, Codex: true, History: true, Processes: false, Load: true, Tokens: true}
}

// agentVisible reports whether the panel for agentID should currently be drawn.
func (s watchSections) agentVisible(agentID string) bool {
	switch agentID {
	case "claude":
		return s.Claude
	case "agy":
		return s.AGY
	case "codex":
		return s.Codex
	case "history":
		return s.History
	case "processes":
		return s.Processes
	default:
		return true
	}
}

// agentKey returns the toggle key shown in an agent panel's own title bar.
func agentKey(agentID string) string {
	switch agentID {
	case "claude":
		return "C"
	case "agy":
		return "G"
	case "codex":
		return "O"
	case "history":
		return "H"
	case "processes":
		return "P"
	default:
		return "?"
	}
}

// buildProcessesBox renders a panel showing active agent processes on the system or remote host.
func buildProcessesBox(width int, counts *AgentProcessCount) wbox {
	title := "\x1b[1m[P]\x1b[0m Processes"
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

// buildLoadBox renders a compact panel showing the system CPU load averages.
func buildLoadBox(width int) wbox {
	title := "\x1b[1m[L]\x1b[0m Load"
	load := CurrentCPULoad()
	if !load.Ok {
		return wbox{title: title, lines: []string{"\x1b[90mload average unavailable\x1b[0m"}, width: width}
	}

	lazyPct := "n/a"
	if load.NumCPU > 0 {
		lazyPct = fmt.Sprintf("%.0f%%", load.Load1/float64(load.NumCPU)*100)
	}
	realPct := "n/a"
	if load.CPUPercentOk {
		realPct = fmt.Sprintf("%.0f%%", load.CPUPercent)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("lazy %s · real %s", lazyPct, realPct))
	lines = append(lines, fmt.Sprintf("avg %.2f  %.2f  %.2f", load.Load1, load.Load5, load.Load15))
	if load.NumCPU > 0 {
		lines = append(lines, fmt.Sprintf("%d cores", load.NumCPU))
	}
	return wbox{title: title, lines: lines, width: width}
}

// buildHistoryBox renders a compact 4th panel showing recorded usage history stats.
func buildHistoryBox(homeDir, historyDir string, width int) wbox {
	title := "\x1b[1m[H]\x1b[0m History"
	targetDir := historyDir
	if targetDir == "" {
		targetDir = HistoryDir(homeDir)
	}

	stats, _ := HistorySummaryStats(targetDir)

	var lines []string
	fileWord := "files"
	if stats.FileCount == 1 {
		fileWord = "file"
	}
	lines = append(lines, fmt.Sprintf("%d %s · %s", stats.FileCount, fileWord, FormatBytes(stats.TotalBytes)))

	// Display directory path shortened with ~ if in user home
	displayDir := targetDir
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(displayDir, home) {
		displayDir = "~" + strings.TrimPrefix(displayDir, home)
	}
	lines = append(lines, displayDir)

	if stats.TotalEntries > 0 {
		var metricParts []string
		if stats.Sparkline != "" {
			metricParts = append(metricParts, "["+stats.Sparkline+"]")
		}
		metricParts = append(metricParts, fmt.Sprintf("+%s used", FormatNumber(stats.TotalUsed)))
		if stats.Duration >= time.Minute {
			metricParts = append(metricParts, fmt.Sprintf("%s/hr", FormatNumber(stats.RatePerHour)))
		}
		lines = append(lines, strings.Join(metricParts, " · "))
	} else {
		lines = append(lines, "\x1b[90mno history recorded yet\x1b[0m")
	}

	return wbox{title: title, lines: lines, width: width}
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

// buildAgentBox renders one agent's status as a compact bordered panel: the
// account/model, at most the two most urgent quota windows, and a token-rate
// sparkline (when showTokens is set). The panel's own toggle key is embedded
// in its title, btop-style, instead of a separate legend. Deliberately terse
// compared to the full `harnez usage` report.
func buildAgentBox(agent AgentUsage, rate agentRate, width int, showTokens, live bool) wbox {
	title := fmt.Sprintf("\x1b[1m[%s]\x1b[0m %s", agentKey(agent.AgentID), agent.Name)

	if !agent.Installed {
		return wbox{title: title, lines: []string{"\x1b[90mnot installed\x1b[0m"}, width: width}
	}
	if !agent.Authenticated {
		return wbox{title: title, lines: []string{"\x1b[90minstalled, not logged in\x1b[0m"}, width: width}
	}

	var lines []string

	acct := agent.Account
	if acct == "" {
		acct = "active session"
	}
	if agent.PlanTier != "" {
		acct += " · " + agent.PlanTier
	}
	lines = append(lines, acct)

	if agent.ActiveModel != "" {
		lines = append(lines, "model: "+agent.ActiveModel)
	}

	var windows []namedWindow
	if agent.Session != nil {
		windows = append(windows, namedWindow{agent.Session.Name, *agent.Session})
	}
	if agent.Weekly != nil {
		windows = append(windows, namedWindow{agent.Weekly.Name, *agent.Weekly})
	}
	for _, mg := range agent.ModelGroups {
		for _, w := range mg.Windows {
			windows = append(windows, namedWindow{mg.Name + " " + w.Name, w})
		}
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].w.UsedPercent > windows[j].w.UsedPercent })

	// contentW: usable characters inside the box borders and padding.
	// renderWBox reserves 4 chars (│·space + space·│), so content = width - 4.
	contentW := width - 4
	if contentW < 10 {
		contentW = 10
	}

	maxShow := 2
	if len(windows) < maxShow {
		maxShow = len(windows)
	}
	for i := 0; i < maxShow; i++ {
		w := windows[i].w
		resetStr := ""
		if w.DurationLeft > 0 {
			resetStr = " · " + FormatDuration(w.DurationLeft)
		}
		label := windows[i].label
		if utf8.RuneCountInString(label) > 16 {
			label = string([]rune(label)[:15]) + "…"
		}
		// Layout: label(16) + " "(1) + bar(barW+2) + " "(1) + percent(6) + resetStr
		// = 26 + barW + visLen(resetStr)
		// Shrink the bar so that resetStr always fits, down to a minimum of 1.
		// Use visLen (not len) so multi-byte runes like · count as 1 column.
		barW := contentW - 26 - visLen(resetStr)
		if barW < 1 {
			// Not enough room for bar + duration: drop duration, keep a stub bar.
			resetStr = ""
			barW = contentW - 26
		}
		if barW < 1 {
			barW = 1
		}
		if barW > 12 {
			barW = 12
		}
		bar := RenderProgressBar(w.UsedPercent, barW)
		lines = append(lines, fmt.Sprintf("%-16s %s %4.1f%%%s", label, bar, w.UsedPercent, resetStr))
	}

	// A fetch error is only worth a line when it actually explains missing
	// quota windows — an agent with model-group windows already showing has
	// nothing to apologize for.
	if agent.QuotaFetchError != "" && len(windows) == 0 {
		lines = append(lines, fmt.Sprintf("\x1b[90mquota: unavailable (%s)\x1b[0m", agent.QuotaFetchError))
	}

	if showTokens && agent.Tokens != nil {
		if live {
			spark := rate.Spark
			if spark == "" {
				spark = "warming up"
			}
			lines = append(lines, fmt.Sprintf("\x1b[1m[T]\x1b[0m tok: %s total · %.0f/min [%s]",
				FormatNumber(agent.Tokens.TotalTokens), rate.PerMinute, spark))
		} else {
			lines = append(lines, fmt.Sprintf("\x1b[1m[T]\x1b[0m tok: %s total",
				FormatNumber(agent.Tokens.TotalTokens)))
		}
	}

	return wbox{title: title, lines: lines, width: width}
}

// gridColumns picks how many panels fit side by side in usable columns, and the
// width each panel gets. The (columns-1) gutters between panels are subtracted
// before dividing, so `columns*boxWidth + gutters` is always <= usable.
func gridColumns(usable, panels int) (columns, boxWidth int) {
	columns = (usable + boxGap) / (minBoxWidth + boxGap)
	if columns < 1 {
		columns = 1
	}
	if columns > panels {
		columns = panels
	}
	boxWidth = (usable - (columns-1)*boxGap) / columns
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}
	return columns, boxWidth
}

// WatchOptions bundles optional customization for watch frame rendering.
type WatchOptions struct {
	Host       string
	ProcCounts *AgentProcessCount
}

// buildWatchFrame lays out one compact, btop-style grid frame: agent panels
// side by side where the terminal is wide enough, filtered by sec, plus a
// footer of toggle badges for each panel and the token line.
//
// The frame is built as discrete lines and budgeted against the terminal's
// height: panel rows are added only while they fit, and a note replaces the
// ones that don't. Overflowing the viewport is what scrolls the terminal and
// desynchronises every later `\x1b[H`, so the layout must never rely on the
// terminal to clip for it.
func buildWatchFrame(summary UsageSummary, rates map[string]agentRate, interval time.Duration, sec watchSections, cols, rows int, live bool, homeDir, historyDir string, opts ...WatchOptions) screenFrame {
	var opt WatchOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	targetHistoryDir := historyDir
	if targetHistoryDir == "" {
		targetHistoryDir = HistoryDir(homeDir)
	}
	fileCount, totalBytes, _ := HistoryStats(targetHistoryDir)
	fileWord := "files"
	if fileCount == 1 {
		fileWord = "file"
	}
	historyStatStr := fmt.Sprintf("   \x1b[90mhistory: %d %s (%s)\x1b[0m", fileCount, fileWord, FormatBytes(totalBytes))

	var visible []AgentUsage
	for _, agent := range summary.Agents {
		if sec.agentVisible(agent.AgentID) {
			visible = append(visible, agent)
		}
	}

	usable := cols - safetyMargin
	if usable > maxTotalWidth {
		usable = maxTotalWidth
	}
	if usable < minTerminalWidth {
		usable = minTerminalWidth
	}

	var hidden []string
	for _, agent := range summary.Agents {
		if !sec.agentVisible(agent.AgentID) {
			hidden = append(hidden, fmt.Sprintf("[%s]", agentKey(agent.AgentID)))
		}
	}
	if !sec.History {
		hidden = append(hidden, "[H]")
	}
	if !sec.Processes {
		hidden = append(hidden, "[P]")
	}
	if !sec.Load {
		hidden = append(hidden, "[L]")
	}
	hiddenHint := ""
	if len(hidden) > 0 {
		hiddenHint = fmt.Sprintf("   \x1b[90mhidden: %s\x1b[0m", strings.Join(hidden, " "))
	}

	titlePrefix := "Agentic usage"
	if opt.Host != "" {
		titlePrefix = fmt.Sprintf("Agentic usage (@%s)", opt.Host)
	}

	header := []string{
		fmt.Sprintf("\x1b[1m%s\x1b[0m  %s%s%s",
			titlePrefix, summary.Timestamp.Format("15:04:05 MST"), historyStatStr, hiddenHint),
		"",
	}
	var footer []string
	if live {
		footer = []string{
			"",
			fmt.Sprintf("refresh every %s   \x1b[90m[a]ll  [r]emote  [q]uit\x1b[0m", interval),
		}
	}

	// Budget for the body: everything the header and footer already claim,
	// plus one spare line for the "panels dropped" note.
	budget := rows - len(header) - len(footer)
	if budget < 1 {
		budget = 1
	}

	totalPanels := len(visible)
	if sec.History {
		totalPanels++
	}
	if sec.Processes {
		totalPanels++
	}
	if sec.Load {
		totalPanels++
	}

	var body []string
	if totalPanels == 0 {
		body = append(body, "\x1b[90m(all panels hidden)\x1b[0m")
	} else {
		columns, boxWidth := gridColumns(usable, totalPanels)

		dropped := 0
		var pending [][]string
		flushRow := func() {
			if len(pending) == 0 {
				return
			}
			lines := combineRow(pending)
			if len(body)+len(lines) <= budget {
				body = append(body, lines...)
			} else {
				dropped += len(pending)
			}
			pending = nil
		}

		for _, agent := range visible {
			box := buildAgentBox(agent, rates[agent.AgentID], boxWidth, sec.Tokens, live)
			pending = append(pending, renderWBox(box))
			if len(pending) == columns {
				flushRow()
			}
		}
		if sec.History {
			hBox := buildHistoryBox(homeDir, historyDir, boxWidth)
			pending = append(pending, renderWBox(hBox))
			if len(pending) == columns {
				flushRow()
			}
		}
		if sec.Processes {
			pBox := buildProcessesBox(boxWidth, opt.ProcCounts)
			pending = append(pending, renderWBox(pBox))
			if len(pending) == columns {
				flushRow()
			}
		}
		if sec.Load {
			lBox := buildLoadBox(boxWidth)
			pending = append(pending, renderWBox(lBox))
			if len(pending) == columns {
				flushRow()
			}
		}
		flushRow()

		if dropped > 0 {
			note := fmt.Sprintf("\x1b[90m… %d panel(s) hidden — terminal too short\x1b[0m", dropped)
			if len(body) < budget {
				body = append(body, note)
			} else if len(body) > 0 {
				body[len(body)-1] = note
			}
		}
	}

	lines := append(append(header, body...), footer...)
	return fit(lines, usable, rows)
}

// RenderSummary prints one static frame of the same compact, btop-style grid
// used by `--watch`, then returns — no alternate screen, no polling loop, no
// keyboard handling. It exists for `harnez usage --summary`: same at-a-glance
// layout as `--watch`, but a plain one-shot print for scripting or a quick
// glance, versus the full `harnez usage` report's per-window detail.
func RenderSummary(ctx context.Context, homeDir string, client *http.Client, out io.Writer, showProcesses ...bool) {
	summary := CollectAll(ctx, homeDir, client)
	cols, rows := terminalSize(out)
	sec := defaultWatchSections()
	if len(showProcesses) > 0 && showProcesses[0] {
		sec.Processes = true
	}
	frame := buildWatchFrame(summary, nil, 0, sec, cols, rows, false, homeDir, "")
	for _, l := range frame.lines {
		fmt.Fprintln(out, l+"\x1b[0m")
	}
}

// RenderSummaryRemote prints one static frame of the compact grid using remote host collection.
func RenderSummaryRemote(ctx context.Context, host string, out io.Writer, showProcesses ...bool) {
	includeProcs := len(showProcesses) > 0 && showProcesses[0]
	summary, procs, _ := CollectRemote(ctx, host, includeProcs)
	cols, rows := terminalSize(out)
	sec := defaultWatchSections()
	if includeProcs {
		sec.Processes = true
	}
	frame := buildWatchFrame(summary, nil, 0, sec, cols, rows, false, "", "", WatchOptions{
		Host:       host,
		ProcCounts: procs,
	})
	for _, l := range frame.lines {
		fmt.Fprintln(out, l+"\x1b[0m")
	}
}

// RunWatch redraws a compact, btop-style usage dashboard in place on a fixed
// interval, instead of the full `harnez usage` report which is too chatty to
// redraw every tick. It polls at most once per interval; interval is clamped
// to MinWatchInterval so `--watch` cannot be used to accidentally hammer live
// quota APIs.
//
// When historyDir is non-empty, every fetched frame is also appended to that
// machine's history log (see AppendHistory) so a `--watch` session builds a
// timeline as it runs, not just at exit.
func RunWatch(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, historyDir string, showProcesses ...bool) error {
	return RunWatchWithHost(ctx, homeDir, client, out, interval, historyDir, "", showProcesses...)
}

// RunWatchWithHost redraws a compact usage dashboard with optional remote host support and [r] toggle.
func RunWatchWithHost(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, historyDir string, initialHost string, showProcesses ...bool) error {
	if interval < MinWatchInterval {
		interval = MinWatchInterval
	}

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	oldState, err := exec.Command("stty", "-F", "/dev/tty", "-g").Output()
	if err == nil {
		_ = exec.Command("stty", "-F", "/dev/tty", "cbreak", "-echo").Run()
		defer func() {
			_ = exec.Command("stty", "-F", "/dev/tty", string(bytes.TrimSpace(oldState))).Run()
		}()
	}

	var secLock sync.Mutex
	sec := defaultWatchSections()
	if len(showProcesses) > 0 && showProcesses[0] {
		sec.Processes = true
	}

	configuredHost := strings.TrimSpace(initialHost)
	activeHost := configuredHost

	fetchChan := make(chan struct{}, 1)
	requestFetch := func() {
		select {
		case fetchChan <- struct{}{}:
		default:
		}
	}

	redrawChan := make(chan struct{}, 1)
	requestRedraw := func() {
		select {
		case redrawChan <- struct{}{}:
		default:
		}
	}

	tty, ttyErr := os.Open("/dev/tty")
	if ttyErr == nil {
		defer tty.Close()
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := tty.Read(buf)
				if err != nil || n == 0 {
					return
				}
				secLock.Lock()
				switch buf[0] {
				case 'c', 'C', '1':
					sec.Claude = !sec.Claude
					requestRedraw()
				case 'g', 'G', '2':
					sec.AGY = !sec.AGY
					requestRedraw()
				case 'o', 'O', '3':
					sec.Codex = !sec.Codex
					requestRedraw()
				case 'h', 'H', '4':
					sec.History = !sec.History
					requestRedraw()
				case 't', 'T', '5':
					sec.Tokens = !sec.Tokens
					requestRedraw()
				case 'p', 'P', '6':
					sec.Processes = !sec.Processes
					requestRedraw()
				case 'l', 'L', '7':
					sec.Load = !sec.Load
					requestRedraw()
				case 'r', 'R':
					if configuredHost != "" {
						if activeHost == "" {
							activeHost = configuredHost
						} else {
							activeHost = ""
						}
						requestFetch()
					}
				case 'a', 'A':
					sec = defaultWatchSections()
					requestRedraw()
				case 'q', 'Q', 3, 27:
					secLock.Unlock()
					stop()
					return
				}
				secLock.Unlock()
			}
		}()
	}

	// Re-lay-out on terminal resize: the frame is budgeted against a concrete
	// width and height, so a stale geometry would either waste space or (worse)
	// overflow the new, smaller viewport.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			requestRedraw()
		}
	}()

	tracker := newRateTracker()

	var lastSummary UsageSummary
	var lastRates map[string]agentRate
	var lastProcs *AgentProcessCount
	var currentHost string

	// Enter the alternate screen buffer, as vim/htop/less do. It gives the
	// redraw loop a viewport with no scrollback of its own, so `\x1b[H` always
	// means the top-left cell the user is looking at, and it restores the
	// user's shell output untouched on exit.
	fmt.Fprint(out, "\033[?1049h\033[?25l\033[2J\033[H")
	defer fmt.Fprint(out, "\033[?25h\033[?1049l")

	draw := func() {
		secLock.Lock()
		activeSec := sec
		secLock.Unlock()

		cols, rows := terminalSize(out)
		buildWatchFrame(lastSummary, lastRates, interval, activeSec, cols, rows, true, homeDir, historyDir, WatchOptions{
			Host:       currentHost,
			ProcCounts: lastProcs,
		}).paint(out)
	}

	renderFrame := func() {
		secLock.Lock()
		targetHost := activeHost
		procRequested := sec.Processes
		secLock.Unlock()

		currentHost = targetHost
		var fresh UsageSummary
		if targetHost != "" {
			var procRes *AgentProcessCount
			fresh, procRes, _ = CollectRemote(sigCtx, targetHost, procRequested)
			lastProcs = procRes
		} else {
			fresh = CollectAll(sigCtx, homeDir, client)
			lastProcs = nil
			if historyDir != "" {
				// Best-effort: a missed append shouldn't interrupt the dashboard.
				_ = AppendHistory(historyDir, fresh)
			}
		}

		lastSummary = applyStaleQuota(fresh, lastSummary)
		lastRates = tracker.update(lastSummary)
		draw()
	}

	renderFrame()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sigCtx.Done():
			return nil
		case <-fetchChan:
			renderFrame()
		case <-redrawChan:
			draw()
		case <-ticker.C:
			renderFrame()
		}
	}
}
