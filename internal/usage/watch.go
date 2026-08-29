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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"ubunatic.com/harnez/internal/rograph"
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
const maxTotalWidth = 120
const preferredAllUsageWidth = 55

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
	AllUsage  bool
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

func compactWatchSections() watchSections {
	return watchSections{AllUsage: true, Load: true, Tokens: true}
}

func applyWatchSectionKey(sec *watchSections, key byte) bool {
	switch key {
	case 'c', 'C', '1':
		sec.Claude = !sec.Claude
	case 'g', 'G', '2':
		sec.AGY = !sec.AGY
	case 'o', 'O', '3':
		sec.Codex = !sec.Codex
	case 'h', 'H', '4':
		sec.History = !sec.History
	case 't', 'T', '5':
		sec.Tokens = !sec.Tokens
	case 'p', 'P', '6':
		sec.Processes = !sec.Processes
	case 'l', 'L', '7':
		sec.Load = !sec.Load
	case 'a':
		sec.AllUsage = !sec.AllUsage
	case 'A':
		*sec = defaultWatchSections()
	default:
		return false
	}
	return true
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

func buildAllUsageBox(summary UsageSummary, width int) wbox {
	lines := allUsageLines(summary, width-4)
	if len(lines) == 0 {
		lines = []string{"\x1b[90mno quota windows available\x1b[0m"}
	}
	return wbox{title: "\x1b[1m[a]\x1b[0m All Usage", lines: lines, width: width}
}

func allUsageLines(summary UsageSummary, contentW int) []string {
	type allUsageRow struct {
		label   string
		windows []QuotaWindow
	}

	var rows []allUsageRow
	labelWidth := 0
	for _, agent := range summary.Agents {
		if !agent.HasUsageData() {
			continue
		}
		if len(agent.ModelGroups) > 0 {
			for _, mg := range agent.ModelGroups {
				label := mg.Name
				if strings.EqualFold(label, "Gemini Models") {
					label = "Gemini"
				} else if strings.EqualFold(label, "Claude and GPT models") || strings.EqualFold(label, "Claude and GPT") {
					label = "Claude/GPT"
				}
				rows = append(rows, allUsageRow{label: label, windows: mg.Windows})
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
			rows = append(rows, allUsageRow{label: agent.Name, windows: wins})
			if n := visLen(agent.Name); n > labelWidth {
				labelWidth = n
			}
		}
	}

	var lines []string
	for _, row := range rows {
		lines = append(lines, formatAllUsageLine(row.label, row.windows, contentW, labelWidth))
	}
	return lines
}

func formatAllUsageLine(label string, windows []QuotaWindow, contentW, labelWidth int) string {
	return formatAllUsageTableLine(label, windows, contentW, labelWidth)
}

func formatAllUsageTableLine(label string, windows []QuotaWindow, contentW, labelWidth int) string {
	if len(windows) < 2 {
		return formatCompactGroupLineWithLabelWidth(label, windows, contentW, labelWidth)
	}

	var w1, w2 QuotaWindow
	foundWeekly, found5h := false, false
	for _, w := range windows {
		nameLow := strings.ToLower(w.Name)
		if !foundWeekly && (strings.Contains(nameLow, "week") || strings.Contains(nameLow, "7-day")) {
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
	if len(d1) > 5 {
		d1 = ""
	}
	if len(d2) > 5 {
		d2 = ""
	}

	b1 := rograph.RenderProgressBar(w1.UsedPercent, 4)
	b2 := rograph.RenderProgressBar(w2.UsedPercent, 4)
	prefix := rograph.PadLabel(label, labelWidth) + "  "

	if d1 != "" && d2 != "" {
		line := prefix + strings.Join([]string{b1, fmt.Sprintf("%.0f%%", w1.UsedPercent), rograph.PadLabel(d1, 5), b2, fmt.Sprintf("%.0f%%", w2.UsedPercent), d2}, " ")
		if visLen(line) <= contentW {
			return line
		}
	}
	if d1 != "" {
		line := prefix + strings.Join([]string{b1, fmt.Sprintf("%.0f%%", w1.UsedPercent), rograph.PadLabel(d1, 5), b2, fmt.Sprintf("%.0f%%", w2.UsedPercent)}, " ")
		if visLen(line) <= contentW {
			return line
		}
	}

	return prefix + strings.Join([]string{b1, fmt.Sprintf("%.0f%%", w1.UsedPercent), b2, fmt.Sprintf("%.0f%%", w2.UsedPercent)}, " ")
}

func compactDurationText(w QuotaWindow) string {
	if w.DurationLeft <= 0 {
		return ""
	}
	return FormatCompactDuration(w.DurationLeft)
}

// buildLoadBox renders a compact panel showing CPU and GPU load, styled
// like the agent quota lines: "label (detail) [spark] avg% (temp)".
func buildLoadBox(width int) wbox {
	title := "\x1b[1m[L]\x1b[0m Load"
	load := CurrentCPULoad()

	var lines []string
	if load.NumCPU > 0 {
		lines = append(lines, formatCPULine(load))
	}
	if load.Memory.Ok {
		lines = append(lines, formatSystemMemoryLine(load.Memory))
	}

	gpus := CurrentGPUs()
	if len(gpus) == 0 {
		lines = append(lines, "\x1b[90mgpu n/a\x1b[0m")
	} else {
		for i, g := range gpus {
			if i >= 2 {
				break
			}
			lines = append(lines, formatGPULine(g))
			if g.HaveMem {
				lines = append(lines, formatGPUMemoryLines(g)...)
			}
		}
	}
	if len(lines) == 0 {
		lines = []string{"\x1b[90mload data unavailable\x1b[0m"}
	}
	return wbox{title: title, lines: lines, width: width}
}

// loadLabelWidth is the fixed column width of the "cpu (...)"/"gpu (...)"
// label in the Load box, so every line's "[spark]" starts at the same
// column regardless of core count or GPU name length.
const loadLabelWidth = 16

// padLoadLabel truncates label to loadLabelWidth (with a trailing "…") if
// it's too long, then pads it to exactly that width. Thin wrapper around
// the shared rograph.PadLabel helper, which also backs buildAgentBox's
// window-label formatting.
func padLoadLabel(label string) string {
	return rograph.PadLabel(label, loadLabelWidth)
}

// formatCPULine renders the "cpu (N cores) [spark] avg% (temp)" line. The
// spark is a timeline of recent aggregate real-time % samples, not a
// per-core snapshot — a spatial snapshot barely changes frame to frame,
// while a trend over the last ~10 samples actually shows something moving.
func formatCPULine(load CPULoad) string {
	series := load.PercentHistory
	if len(series) == 0 {
		val := 0.0
		if load.CPUPercentOk {
			val = load.CPUPercent
		}
		series = []float64{val}
	}
	avgPart := "n/a"
	if load.CPUPercentOk {
		avgPart = fmt.Sprintf("%.0f%%", load.CPUPercent)
	}
	tempPart := ""
	if load.TempOk {
		tempPart = fmt.Sprintf(" (%.0f°C)", load.TempC)
	}
	label := padLoadLabel(fmt.Sprintf("cpu (%d cores)", load.NumCPU))
	return fmt.Sprintf("%s [%s] %s%s", label, rograph.PercentSparkline(series, min(rograph.MaxWidth, len(series))), avgPart, tempPart)
}

func formatSystemMemoryLine(mem SystemMemory) string {
	label := padLoadLabel("ram")
	return fmt.Sprintf("%s %s/%sG %.0f%%", label, formatGiB(mem.UsedMiB), formatGiB(mem.TotalMiB), percent(mem.UsedMiB, mem.TotalMiB))
}

// formatGPULine renders the "gpu (name) [spark] avg% (temp)" line. The
// sparkline shows recent history (UtilHistory), or degenerates to a single
// current-value glyph if no history has been collected yet.
func formatGPULine(g GPU) string {
	series := g.UtilHistory
	if len(series) == 0 {
		series = []float64{g.UtilPercent}
	}
	label := padLoadLabel(fmt.Sprintf("gpu (%s)", g.Name))
	tempPart := ""
	if g.HaveTemp {
		tempPart = fmt.Sprintf(" (%.0f°C)", g.TempC)
	}
	return fmt.Sprintf("%s [%s] %.0f%%%s", label, rograph.PercentSparkline(series, min(rograph.MaxWidth, len(series))), g.UtilPercent, tempPart)
}

func formatGPUMemoryLines(g GPU) []string {
	label := padLoadLabel("gpu mem")
	lines := []string{fmt.Sprintf("%s %s/%sG %.0f%%", label, formatGiB(g.MemUsedMiB), formatGiB(g.MemTotalMiB), g.MemPercent)}

	parts := []string{}
	if g.HaveVRAM {
		parts = append(parts, fmt.Sprintf("v%s/%s", formatGiB(g.VRAMUsedMiB), formatGiB(g.VRAMTotalMiB)))
	}
	if g.HaveGTT {
		parts = append(parts, fmt.Sprintf("g%s/%s", formatGiB(g.GTTUsedMiB), formatGiB(g.GTTTotalMiB)))
	}
	if len(parts) > 0 {
		lines = append(lines, fmt.Sprintf("%s %s", padLoadLabel("vram/gtt"), strings.Join(parts, " ")))
	}
	return lines
}

func formatGiB(mib float64) string {
	return fmt.Sprintf("%.1f", mib/1024)
}

func percent(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
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

	// contentW: usable characters inside the box borders and padding.
	// renderWBox reserves 4 chars (│·space + space·│), so content = width - 4.
	contentW := width - 4
	if contentW < 10 {
		contentW = 10
	}

	// Render quota lines. If the agent has ModelGroups (e.g. AGY), render each group
	// as a compact twin-quota line: Label [Bar1] [Bar2] Pct1 Dt1 Pct2 Dt2.
	// Otherwise, if the agent has Session and/or Weekly, render them compactly together or individually.
	if len(agent.ModelGroups) > 0 {
		for _, mg := range agent.ModelGroups {
			label := mg.Name
			if strings.EqualFold(label, "Gemini Models") {
				label = "Gemini"
			} else if strings.EqualFold(label, "Claude and GPT models") || strings.EqualFold(label, "Claude and GPT") {
				label = "Claude/GPT"
			}
			line := formatCompactGroupLine(label, mg.Windows, contentW)
			lines = append(lines, line)
		}
	} else if agent.Session != nil || agent.Weekly != nil {
		var wins []QuotaWindow
		// Order: Weekly first, then Session (or vice-versa, matching weekly/5h)
		if agent.Weekly != nil {
			wins = append(wins, *agent.Weekly)
		}
		if agent.Session != nil {
			wins = append(wins, *agent.Session)
		}
		if len(wins) == 2 {
			label := "Wk / 5h"
			line := formatCompactGroupLine(label, wins, contentW)
			lines = append(lines, line)
		} else if len(wins) == 1 {
			line := formatCompactGroupLine(wins[0].Name, wins, contentW)
			lines = append(lines, line)
		}
	}

	// A fetch error is only worth a line when it actually explains missing
	// quota windows — an agent with model-group windows already showing has
	// nothing to apologize for.
	if agent.QuotaFetchError != "" && len(agent.ModelGroups) == 0 && agent.Session == nil && agent.Weekly == nil {
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
		bar := rograph.RenderProgressBar(w.UsedPercent, 4)
		line := fmt.Sprintf("%s %s %3.0f%%%s", lbl, bar, w.UsedPercent, resetStr)
		if visLen(line) > contentW {
			line = fmt.Sprintf("%s %s %3.0f%%", lbl, bar, w.UsedPercent)
		}
		return line
	}

	// 2 or more windows: sort/pick Weekly and Short-term (5-Hour) or first 2
	var w1, w2 QuotaWindow
	// Find weekly and 5h windows if possible
	foundWeekly, found5h := false, false
	for _, w := range windows {
		nameLow := strings.ToLower(w.Name)
		if !foundWeekly && (strings.Contains(nameLow, "week") || strings.Contains(nameLow, "7-day")) {
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

	d1 := ""
	if w1.DurationLeft > 0 {
		d1 = " " + FormatCompactDuration(w1.DurationLeft)
	}
	d2 := ""
	if w2.DurationLeft > 0 {
		d2 = " " + FormatCompactDuration(w2.DurationLeft)
	}

	lbl := rograph.PadLabel(label, labelWidth)
	b1 := rograph.RenderProgressBar(w1.UsedPercent, 4)
	b2 := rograph.RenderProgressBar(w2.UsedPercent, 4)

	// Format: Label [b1] pct1 d1 [b2] pct2 d2 (e.g. Gemini [███░] 91% 2h [░░░░] 0% 3d)
	line := fmt.Sprintf("%s %s %.0f%%%s %s %.0f%%%s", lbl, b1, w1.UsedPercent, d1, b2, w2.UsedPercent, d2)
	if visLen(line) <= contentW {
		return line
	}

	// Drop d2 if too long
	line = fmt.Sprintf("%s %s %.0f%%%s %s %.0f%%", lbl, b1, w1.UsedPercent, d1, b2, w2.UsedPercent)
	if visLen(line) <= contentW {
		return line
	}

	// Drop d1 as well
	line = fmt.Sprintf("%s %s %.0f%% %s %.0f%%", lbl, b1, w1.UsedPercent, b2, w2.UsedPercent)
	if visLen(line) <= contentW {
		return line
	}

	// If still too long in very narrow box, shrink label
	for lw := labelWidth - 1; lw >= 6; lw-- {
		lblShrunk := rograph.PadLabel(label, lw)
		line = fmt.Sprintf("%s %s %.0f%% %s %.0f%%", lblShrunk, b1, w1.UsedPercent, b2, w2.UsedPercent)
		if visLen(line) <= contentW {
			return line
		}
	}

	// If still too long, shrink label to fit
	return line
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
	Host          string
	ProcCounts    *AgentProcessCount
	Compact       bool
	ShowProcesses bool
}

func initialWatchSections(opts WatchOptions) watchSections {
	if opts.Compact {
		return compactWatchSections()
	}
	sec := defaultWatchSections()
	if opts.ShowProcesses {
		sec.Processes = true
	}
	return sec
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

	// discovered is every agent the collector actually found real local/
	// remote state for (issue 083: self-hiding, auto-discovery display) — an
	// agent that isn't installed or configured on this machine never enters
	// the toggle/visibility machinery below at all, so it can't show up as
	// an empty box or a "hidden: [x]" toggle hint either.
	var discovered []AgentUsage
	for _, agent := range summary.Agents {
		if agent.HasUsageData() {
			discovered = append(discovered, agent)
		}
	}

	var visible []AgentUsage
	for _, agent := range discovered {
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
	for _, agent := range discovered {
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
	if !sec.AllUsage {
		hidden = append(hidden, "[a]")
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
			fmt.Sprintf("refresh every %s   \x1b[90m[a]usage  [⇧a]ll  [r]emote  [q]uit\x1b[0m", interval),
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
	// No agent had any real recorded usage found on this machine — say so
	// explicitly rather than silently rendering a screen with no agent
	// boxes at all (issue 083).
	if len(discovered) == 0 && len(summary.Agents) > 0 {
		body = append(body,
			"\x1b[90mno agent usage detected — install/configure Claude Code, Codex, or AGY\x1b[0m",
			"\x1b[90mor run `harnez agent-collector --once` to collect a fresh snapshot\x1b[0m",
			"",
		)
	}
	dropped := 0
	renderedCompactPair := false
	if onlyAllUsageAndLoad(sec, visible) && usable >= preferredAllUsageWidth+boxGap+minBoxWidth {
		allWidth := preferredAllUsageWidth
		loadWidth := usable - allWidth - boxGap
		if loadWidth < minBoxWidth {
			loadWidth = minBoxWidth
			allWidth = usable - loadWidth - boxGap
		}
		lines := combineRow([][]string{
			renderWBox(buildAllUsageBox(summary, allWidth)),
			renderWBox(buildLoadBox(loadWidth)),
		})
		if len(body)+len(lines) <= budget {
			body = append(body, lines...)
		} else {
			dropped += 2
		}
		renderedCompactPair = true
		totalPanels = 0
	}
	if !renderedCompactPair && sec.AllUsage {
		lines := renderWBox(buildAllUsageBox(summary, usable))
		if len(body)+len(lines) <= budget {
			body = append(body, lines...)
		} else {
			dropped++
		}
	}
	if totalPanels == 0 {
		if !sec.AllUsage && !renderedCompactPair {
			body = append(body, "\x1b[90m(all panels hidden)\x1b[0m")
		}
	} else {
		columns, boxWidth := gridColumns(usable, totalPanels)

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
	}

	if dropped > 0 {
		note := fmt.Sprintf("\x1b[90m… %d panel(s) hidden — terminal too short\x1b[0m", dropped)
		if len(body) < budget {
			body = append(body, note)
		} else if len(body) > 0 {
			body[len(body)-1] = note
		}
	}

	lines := append(append(header, body...), footer...)
	return fit(lines, usable, rows)
}

func onlyAllUsageAndLoad(sec watchSections, visible []AgentUsage) bool {
	return sec.AllUsage && sec.Load && len(visible) == 0 && !sec.History && !sec.Processes
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
	opts := WatchOptions{Host: initialHost}
	if len(showProcesses) > 0 && showProcesses[0] {
		opts.ShowProcesses = true
	}
	return RunWatchWithOptions(ctx, homeDir, client, out, interval, historyDir, opts)
}

func RunWatchWithOptions(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, historyDir string, opts WatchOptions) error {
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
	sec := initialWatchSections(opts)

	configuredHost := strings.TrimSpace(opts.Host)
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
				if applyWatchSectionKey(&sec, buf[0]) {
					requestRedraw()
				} else {
					switch buf[0] {
					case 'r', 'R':
						if configuredHost != "" {
							if activeHost == "" {
								activeHost = configuredHost
							} else {
								activeHost = ""
							}
							requestFetch()
						}
					case 'q', 'Q', 3, 27:
						secLock.Unlock()
						stop()
						return
					}
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

	// The Load panel's CPU/GPU numbers come from local /proc and sysfs
	// reads only (no subprocess involved at all), not the network-backed
	// quota fetch that `interval` paces, so it redraws on its own faster
	// cadence via the existing draw()/redrawChan path rather than
	// triggering a full renderFrame(). 1s keeps the sparkline visibly live
	// without the flicker/noise of a sub-second cadence.
	loadTicker := time.NewTicker(time.Second)
	defer loadTicker.Stop()

	for {
		select {
		case <-sigCtx.Done():
			return nil
		case <-fetchChan:
			renderFrame()
		case <-redrawChan:
			draw()
		case <-loadTicker.C:
			draw()
		case <-ticker.C:
			renderFrame()
		}
	}
}
