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
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"ubunatic.com/harnez/internal/rograph"
	"ubunatic.com/harnez/internal/uix"
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

// compactBarWidth is the fixed cell width of a quota/usage bar rendered
// inline in a compact row (all-usage rows, model-group rows, etc.), as
// opposed to a full box's own bar width. Named so the seven call sites that
// need it stay in lockstep -- see issue 223.
const compactBarWidth = 4

// maxPanelContentWidth caps every panel's *declared* preferred width, so
// truncation is a single deliberate policy applied consistently across all
// panel types (Claude/AGY/Codex, All Usage, History, Processes, Load)
// instead of a per-panel special case (issue 093). A panel whose natural
// content is wider than this gets truncated by renderWBox's ellipsis, the
// same way every other panel does.
const maxPanelContentWidth = 80

// measureWidth is the generous width used to measure a panel's *natural*
// content width before layout: wide enough that no build*Box function's
// internal graceful-degradation logic (e.g. dropping a duration label) ever
// triggers, so the measured width reflects the panel's actual full-detail
// content, not an accidental truncation artifact.
const measureWidth = 200

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
	Mic       bool
	Tokens    bool
}

func defaultWatchSections() watchSections {
	// Mic starts off like Processes (issue 244): a small opt-in status box
	// rather than a permanent panel, per issue 085's "keep it minimal by
	// default" precedent for this kind of box.
	return watchSections{Claude: true, AGY: true, Codex: true, History: true, Processes: false, Load: true, Mic: false, Tokens: true}
}

func compactWatchSections() watchSections {
	return watchSections{AllUsage: true, Load: true, Tokens: true}
}

// agentsOnlyWatchSections shows only the discovered per-agent boxes plus
// token detail, with History/Load/Processes/AllUsage all off — the
// "Agents-only" preset from issue 094's proposal section 4.
func agentsOnlyWatchSections() watchSections {
	return watchSections{Claude: true, AGY: true, Codex: true, Tokens: true}
}

// watchPresetOrder and watchPresetNames define the fixed cycle order for the
// [m] mode/preset shortcut (issue 094): default -> compact -> agents ->
// default. Cycling is keyed off an explicit index rather than sniffing sec's
// current field values, since sec can drift away from any named preset via
// individual panel toggles ([C]/[a]/etc.) between presses.
var watchPresetOrder = []watchSections{
	defaultWatchSections(),
	compactWatchSections(),
	agentsOnlyWatchSections(),
}

var watchPresetNames = []string{"default", "compact", "agents"}

// nextWatchPreset returns the section set, name, and new index for the
// preset that follows idx in watchPresetOrder, wrapping around.
func nextWatchPreset(idx int) (watchSections, string, int) {
	n := (idx + 1) % len(watchPresetOrder)
	return watchPresetOrder[n], watchPresetNames[n], n
}

// watchKeyState is the mutable interactive state a single keypress can
// change in RunWatchWithOptions's tty-reading goroutine. Threading it
// through a pure function (dispatchWatchKey) keeps that goroutine a thin
// shell around testable logic instead of embedding the overlay/preset
// dispatch rules inline where only a real PTY could exercise them.
type watchKeyState struct {
	sec            watchSections
	overlayOpen    bool
	presetIdx      int
	configuredHost string
	activeHost     string
	// debugOverlay is issue 131's `!`-toggled per-agent freshness countdown
	// overlay: in-process only, never persisted, independent of overlayOpen
	// (the [?] Controls reference overlay).
	debugOverlay bool
}

// watchKeyEffect reports what the caller should do in response to one
// keypress: redraw the current frame, kick off a fresh data fetch, or quit.
type watchKeyEffect struct {
	redraw bool
	fetch  bool
	quit   bool
}

// dispatchWatchKey applies one keypress to st and returns the updated state
// plus the side effect the caller owes (issue 094). Key semantics, in
// priority order:
//
//  1. [?] always toggles the Controls overlay open/closed, regardless of
//     any other state.
//  2. While the overlay is open, every key is swallowed except its
//     documented dismiss keys (Esc, q/Q, Ctrl-C, Enter) — a stray
//     panel-toggle press must not silently change state behind the overlay.
//  3. [m]/[M] cycles the default -> compact -> agents-only -> default
//     preset, re-applying showProcesses so an explicit --proc request keeps
//     winning across preset changes the same way initialWatchSections
//     already makes it win at startup.
//  4. [!] toggles the debug overlay (issue 131's per-agent freshness
//     countdown gauge) — in-process only, never persisted.
//  5. Existing single-key panel toggles (applyWatchSectionKey) keep working
//     unchanged, for backward compatibility.
//  6. [r]/[R] flips remote/local when a host was configured; [q]/[Q]/
//     Ctrl-C/Esc quits.
func dispatchWatchKey(st watchKeyState, key byte, showProcesses bool) (watchKeyState, watchKeyEffect) {
	action := mustWatchActions().actionForKey(key)

	switch {
	case action == "toggle_controls":
		st.overlayOpen = !st.overlayOpen
		return st, watchKeyEffect{redraw: true}

	case st.overlayOpen:
		// Dismiss keys are deliberately not spec-driven: Enter closes the
		// overlay but isn't a top-level hotkey shown anywhere else (title,
		// footer, or overlay body), so it has no display symbol to source
		// from spec/actions.yaml.
		switch key {
		case 'q', 'Q', 3, 27, '\r', '\n':
			st.overlayOpen = false
			return st, watchKeyEffect{redraw: true}
		}
		return st, watchKeyEffect{}

	case action == "cycle_preset":
		sec, _, idx := nextWatchPreset(st.presetIdx)
		if showProcesses {
			sec.Processes = true
		}
		st.sec, st.presetIdx = sec, idx
		return st, watchKeyEffect{redraw: true}

	case action == "toggle_debug_overlay":
		st.debugOverlay = !st.debugOverlay
		return st, watchKeyEffect{redraw: true}

	case applyWatchSectionKey(&st.sec, key):
		return st, watchKeyEffect{redraw: true}

	default:
		switch action {
		case "toggle_remote":
			if st.configuredHost != "" {
				if st.activeHost == "" {
					st.activeHost = st.configuredHost
				} else {
					st.activeHost = ""
				}
				return st, watchKeyEffect{fetch: true}
			}
		case "quit":
			return st, watchKeyEffect{quit: true}
		}
	}
	return st, watchKeyEffect{}
}

// splashKeyEffect reports what the caller should do in response to one
// keypress received while the startup splash (issue 164) is showing.
type splashKeyEffect struct {
	skip bool // abort only the splash wait; must NOT quit
	quit bool // Ctrl-C is the universal interrupt, even during splash
}

// dispatchSplashKey applies one keypress received while the startup splash
// is active. It is deliberately narrower than, and separate from,
// dispatchWatchKey: byte 27 (Esc) here only skips the splash wait -- it must
// NOT quit, which is a deliberate deviation from Esc's meaning on the main
// dashboard once the splash has ended (dispatchWatchKey still owns that
// unchanged). Ctrl-C (byte 3) quits either way, since it is the universal
// interrupt. Every other key is ignored during splash -- the dashboard
// (sec/overlayOpen/etc.) is not showing yet, so panel toggles have nothing
// to act on.
func dispatchSplashKey(key byte) splashKeyEffect {
	switch key {
	case 3:
		return splashKeyEffect{quit: true}
	case 27:
		return splashKeyEffect{skip: true}
	}
	return splashKeyEffect{}
}

// applyWatchSectionKey applies key to sec if it maps (via spec/actions.yaml)
// to a box toggle or the panel-reset action, and reports whether it did.
// The key->action *mapping* comes from the embedded spec; which
// watchSections field each action flips is dispatch logic and stays here.
func applyWatchSectionKey(sec *watchSections, key byte) bool {
	switch mustWatchActions().actionForKey(key) {
	case "toggle_all_usage":
		sec.AllUsage = !sec.AllUsage
	case "toggle_claude":
		sec.Claude = !sec.Claude
	case "toggle_agy":
		sec.AGY = !sec.AGY
	case "toggle_codex":
		sec.Codex = !sec.Codex
	case "toggle_history":
		sec.History = !sec.History
	case "toggle_processes":
		sec.Processes = !sec.Processes
	case "toggle_load":
		sec.Load = !sec.Load
	case "toggle_mic":
		sec.Mic = !sec.Mic
	case "toggle_tokens":
		sec.Tokens = !sec.Tokens
	case "reset_default":
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

// agentKey returns a short internal box-ID abbreviation used only for the
// "hidden — terminal too short" drop note (buildWatchFrame) and uix.Box
// identity. It is not a keyboard hotkey and not spec-driven: the actual
// toggle keys and their display symbols for these boxes live in
// spec/actions.yaml and are looked up via watchBoxSymbol for title
// rendering instead.
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
	lines := allUsageLinesAt(summary, width-4, debugOverlay, now, refreshInterval)
	if len(lines) == 0 {
		lines = []string{ansiDimGrey + "no quota windows available\x1b[0m"}
	}
	return wbox{title: watchBoxSymbol("all_usage") + " All Usage", lines: lines, width: width}
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
	for _, agent := range summary.Agents {
		if !agent.HasUsageData() {
			continue
		}
		// Issue 107: staleness is a per-agent-row verdict (the data model has
		// no per-QuotaWindow provenance yet — see AgentUsage.IsValueStale's
		// doc comment), so every row this agent contributes (its model
		// groups included) shares the one flag.
		stale := agent.IsValueStale()
		if len(agent.ModelGroups) > 0 {
			for _, mg := range agent.ModelGroups {
				label := mg.Name
				if strings.EqualFold(label, "Gemini Models") {
					label = "Gemini"
				} else if strings.EqualFold(label, "Claude and GPT models") || strings.EqualFold(label, "Claude and GPT") {
					label = "Claude/GPT"
				}
				rows = append(rows, allUsageRow{label: label, windows: mg.Windows, lastRefreshed: agent.LastRefreshed, stale: stale})
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
			rows = append(rows, allUsageRow{label: agent.Name, windows: wins, lastRefreshed: agent.LastRefreshed, stale: stale})
			if n := visLen(agent.Name); n > labelWidth {
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
		if strings.Contains(name, "week") || strings.Contains(name, "7-day") {
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
	if w.DurationLeft <= 0 {
		return ""
	}
	if w.DurationLeft >= 24*time.Hour && w.DurationLeft < 48*time.Hour {
		days := int(w.DurationLeft.Hours()) / 24
		hours := int(w.DurationLeft.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	return FormatCompactDuration(w.DurationLeft)
}

// buildLoadBox renders a compact panel showing CPU and GPU load, styled
// like the agent quota lines: "label (detail) [spark] avg% (temp)".
//
// remoteHost and snapshot together select the data source (issue 090): with
// remoteHost == "", this is local mode and CurrentCPULoad()/CurrentGPUs()
// are called directly, giving the 1Hz-redraw live behavior local mode has
// always had. With remoteHost != "", this is remote mode: the panel renders
// exclusively from snapshot (last fetched at the remote collection
// interval, not the 1Hz redraw cadence — no local /proc or /sys read is
// substituted, and no SSH call happens here). A nil snapshot in remote mode
// renders an explicit "remote load unavailable" placeholder rather than
// silently falling back to local telemetry.
func buildLoadBox(width int, remoteHost string, snapshot *LoadSnapshot) wbox {
	title := watchBoxSymbol("load") + " Load"
	if remoteHost != "" {
		title = fmt.Sprintf("%s Load %s(@%s)\x1b[0m", watchBoxSymbol("load"), ansiDimGrey, remoteHost)
	}
	return wbox{title: title, lines: buildLoadBoxLines(remoteHost, snapshot), width: width}
}

// buildRemoteLoadBox renders issue 110's independent remote Load box: the
// CPU/GPU load of load.watch_host, a separate panel from the existing local
// (or --host-retitled) [L] Load box above — not merged into it (Decision
// §5). Shares buildLoadBox's row-rendering via buildLoadBoxLines; only the
// title/panel key differ. host must be non-empty — callers only build this
// panel at all when load.watch_host is configured (Acceptance Criterion 1).
//
// The title also carries a "streaming"/"batch" label so it's always visible
// whether the current data arrived over the persistent --watch-only stream
// or a one-shot poll (issue 110's follow-up UI request) — never hidden
// state the user has to infer from update cadence.
func buildRemoteLoadBox(width int, host string, snapshot *LoadSnapshot, streaming bool) wbox {
	mode := "batch"
	if streaming {
		mode = "streaming"
	}
	title := fmt.Sprintf("%s[R]\x1b[0m Remote Load %s(@%s · %s)\x1b[0m", ansiBold, ansiDimGrey, host, mode)
	return wbox{title: title, lines: buildLoadBoxLines(host, snapshot), width: width}
}

// buildLoadBoxLines renders the shared CPU/GPU row content for both
// buildLoadBox and buildRemoteLoadBox.
//
// remoteHost and snapshot together select the data source (issue 090): with
// remoteHost == "", this is local mode and CurrentCPULoad()/CurrentGPUs()
// are called directly, giving the 1Hz-redraw live behavior local mode has
// always had. With remoteHost != "", this is remote mode: the panel renders
// exclusively from snapshot (last fetched at the remote collection
// interval, not the 1Hz redraw cadence — no local /proc or /sys read is
// substituted, and no SSH call happens here). A nil snapshot in remote mode
// renders an explicit "remote load unavailable" placeholder rather than
// silently falling back to local telemetry.
func buildLoadBoxLines(remoteHost string, snapshot *LoadSnapshot) []string {
	if remoteHost != "" && snapshot == nil {
		return []string{ansiDimGrey + "remote load unavailable\x1b[0m"}
	}

	var load CPULoad
	var gpus []GPU
	if snapshot != nil {
		load = snapshot.CPU
		gpus = snapshot.GPUs
	} else {
		load = CurrentCPULoad()
		gpus = CurrentGPUs()
	}

	var lines []string
	if load.NumCPU > 0 {
		lines = append(lines, formatCPULine(load))
	}
	if load.Memory.Ok {
		lines = append(lines, formatSystemMemoryLine(load.Memory))
	}

	if len(gpus) == 0 {
		lines = append(lines, ansiDimGrey+"gpu n/a\x1b[0m")
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
		lines = []string{ansiDimGrey + "load data unavailable\x1b[0m"}
	}
	return lines
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

// formatCPULine renders the "cpu (N cores) [<chart>] avg% (temp)" line. The
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
	series = padHistory(series, load.CPUPercent, loadHistoryLen)
	avgPart := "n/a"
	if load.CPUPercentOk {
		avgPart = watchLoadPercent(load.CPUPercent)
	}
	tempPart := ""
	if load.TempOk {
		tempPart = fmt.Sprintf(" (%.0f°C)", load.TempC)
	}
	label := padLoadLabel(fmt.Sprintf("cpu (%d cores)", load.NumCPU))

	var chart string
	mode := mustIndicators().LoadCharts.CPUMode()
	if mode == LoadChartBar {
		val := 0.0
		if load.CPUPercentOk {
			val = load.CPUPercent
		}
		chart = rograph.RenderBar(val, watchLoadBarOptions(val))
	} else {
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	}
	return fmt.Sprintf("%s %s %s%s", label, chart, avgPart, tempPart)
}

func padHistory(series []float64, current float64, width int) []float64 {
	if len(series) >= width {
		return series
	}
	fillVal := current
	if len(series) > 0 {
		fillVal = series[0]
	}
	res := make([]float64, width)
	padCount := width - len(series)
	for i := 0; i < padCount; i++ {
		res[i] = fillVal
	}
	copy(res[padCount:], series)
	return res
}

func formatSystemMemoryLine(mem SystemMemory) string {
	label := FormatRAMLabel(mem)
	pct := percent(mem.UsedMiB, mem.TotalMiB)

	var chart string
	mode := mustIndicators().LoadCharts.RAMMode()
	if mode != LoadChartBar {
		series := padHistory(mem.PercentHistory, pct, rograph.MaxWidth*2)
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	} else {
		chart = rograph.RenderBar(pct, watchLoadBarOptions(pct))
	}

	return fmt.Sprintf("%s %s %s/%sG %s", label, chart, formatGiB(mem.UsedMiB), formatGiB(mem.TotalMiB), watchLoadPercent(pct))
}

// formatGPULine renders the "gpu (name) [<chart>] avg% (temp)" line. The
// sparkline shows recent history (UtilHistory), or degenerates to a single
// current-value glyph if no history has been collected yet.
func formatGPULine(g GPU) string {
	series := g.UtilHistory
	if len(series) == 0 {
		series = []float64{g.UtilPercent}
	}
	series = padHistory(series, g.UtilPercent, loadHistoryLen)
	label := padLoadLabel(fmt.Sprintf("gpu (%s)", g.Name))
	tempPart := ""
	if g.HaveTemp {
		tempPart = fmt.Sprintf(" (%.0f°C)", g.TempC)
	}

	var chart string
	mode := mustIndicators().LoadCharts.GPUMode()
	if mode == LoadChartBar {
		chart = rograph.RenderBar(g.UtilPercent, watchLoadBarOptions(g.UtilPercent))
	} else {
		chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
	}
	return fmt.Sprintf("%s %s %s%s", label, chart, watchLoadPercent(g.UtilPercent), tempPart)
}

// formatGPUMemoryLines renders GPU memory as a single "gpu vram/gtt" row
// combining VRAM and GTT into adjacent compact bars/sparklines (issue 089, issue 198),
// replacing the earlier two plain-text rows (gpu mem / vram/gtt). It mirrors
// formatAllUsageTableLine's adjacent-dual-bar pattern rather than hand-rolling a new bar renderer.
//
// g.MemUsedMiB/MemTotalMiB/MemPercent already hold the VRAM+GTT combined
// totals (see readAMDGPUMemory in load.go, which accumulates into them from
// whichever of HaveVRAM/HaveGTT is set), so they're used directly here for
// the combined used/total and percentage rather than re-summing.
func formatGPUMemoryLines(g GPU) []string {
	label := padLoadLabel("gpu vram/gtt")

	var chart string
	if g.HaveVRAM && g.HaveGTT {
		vramPct := percent(g.VRAMUsedMiB, g.VRAMTotalMiB)
		gttPct := percent(g.GTTUsedMiB, g.GTTTotalMiB)
		mode := mustIndicators().LoadCharts.VRAMMode()
		if mode != LoadChartBar {
			vramSeries := padHistory(g.VRAMPercentHistory, vramPct, 8)
			gttSeries := padHistory(g.GTTPercentHistory, gttPct, 8)
			chart = fmt.Sprintf("[%s][%s]", watchPercentSparkline(vramSeries, 4, mode), watchPercentSparkline(gttSeries, 4, mode))
		} else {
			barOpts := watchLoadBarOptions(vramPct)
			barOpts.Width = compactBarWidth
			chart = rograph.RenderBar(vramPct, barOpts)
			barOpts.ForegroundANSI = ""
			if mustIndicators().chartPresentation() == LoadChartHeat {
				barOpts.ForegroundANSI = heatForegroundANSI(gttPct)
			}
			chart += rograph.RenderBar(gttPct, barOpts)
		}
	} else {
		var activePct float64
		var activeHist []float64
		if g.HaveVRAM {
			activePct = percent(g.VRAMUsedMiB, g.VRAMTotalMiB)
			activeHist = g.VRAMPercentHistory
		} else if g.HaveGTT {
			activePct = percent(g.GTTUsedMiB, g.GTTTotalMiB)
			activeHist = g.GTTPercentHistory
		} else {
			activePct = g.MemPercent
		}

		mode := mustIndicators().LoadCharts.VRAMMode()
		if mode != LoadChartBar {
			series := padHistory(activeHist, activePct, rograph.MaxWidth*2)
			chart = fmt.Sprintf("[%s]", watchPercentSparkline(series, rograph.MaxWidth, mode))
		} else {
			chart = rograph.RenderBar(activePct, watchLoadBarOptions(activePct))
		}
	}

	line := fmt.Sprintf("%s %s %s/%sG %s", label, chart, formatGiB(g.MemUsedMiB), formatGiB(g.MemTotalMiB), watchLoadPercent(g.MemPercent))
	return []string{line}
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

// buildMicBox renders issue 244's compact panel: the default system
// microphone's input level and whether anything is currently recording
// from it. st is resolved once per redraw frame by the caller (see the
// micStatus comment in buildWatchFrameAt) so the panel-measure/panel-render
// double-build that every panel gets doesn't spawn the underlying pactl/
// amixer subprocess twice.
func buildMicBox(width int, st MicStatus) wbox {
	title := watchBoxSymbol("mic") + " Mic"
	return wbox{title: title, lines: buildMicBoxLines(st), width: width}
}

// buildMicBoxLines renders MicStatus as two lines — the configured-gain bar
// (issue 244) and, below it, the live peak/RMS reading (issue 245) or an
// "n/a" placeholder when live capture isn't available — or a single dim
// "unavailable" placeholder when no audio interface was found at all
// (Available == false) — the box itself is never added to the panel list
// in that case (see buildWatchFrameAt), but this keeps buildMicBoxLines
// safe to call standalone, e.g. from tests.
func buildMicBoxLines(st MicStatus) []string {
	if !st.Available {
		return []string{ansiDimGrey + "mic unavailable\x1b[0m"}
	}

	bar := rograph.RenderBar(st.Level, rograph.BarOptions{IncludePercent: true, PercentPrecision: 0})

	recordingWord := "off"
	if st.Recording {
		recordingWord = "on"
	}
	if st.Backend == "amixer" {
		// ALSA has no generic "who's holding this device open" signal the
		// way PipeWire/PulseAudio's source-outputs list does (see
		// currentMicStatusAmixer) — say so rather than implying "off" is an
		// actual observation.
		recordingWord = "n/a"
	}

	line := fmt.Sprintf("%s   recording %s", bar, recordingWord)
	if st.Muted {
		line += ansiDimGrey + "  (muted)\x1b[0m"
	}

	// Issue 245: a second line for the genuine live signal reading,
	// alongside (not replacing) the gain bar above — "gain" answers "is the
	// mic turned up", "live" answers "is sound actually reaching it right
	// now". Rendered as "n/a" rather than a fabricated bar when the capture
	// subprocess isn't available (no parec, amixer backend, or still
	// (re)connecting) — see micLiveManager's doc comment for when that is.
	var liveLine string
	if st.LiveAvailable {
		liveBar := rograph.RenderBar(st.LiveLevel, rograph.BarOptions{IncludePercent: true, PercentPrecision: 0})
		liveLine = fmt.Sprintf("%s   live", liveBar)
	} else {
		liveLine = ansiDimGrey + "live n/a" + "\x1b[0m"
	}

	return []string{line, liveLine}
}

// buildHistoryBox renders a compact 4th panel showing recorded usage history stats.
func buildHistoryBox(homeDir, historyDir string, width int) wbox {
	title := watchBoxSymbol("history") + " History"
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
		lines = append(lines, ansiDimGrey+"no history recorded yet\x1b[0m")
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
func buildAgentBox(agent AgentUsage, rate agentRate, width int, showTokens, live, debugOverlay bool) wbox {
	return buildAgentBoxAt(agent, rate, width, showTokens, live, debugOverlay, time.Now(), DefaultWatchInterval)
}

// buildAgentBoxAt is buildAgentBox with the redraw timestamp supplied by the
// caller, keeping all freshness gauges in a watch frame consistent.
func buildAgentBoxAt(agent AgentUsage, rate agentRate, width int, showTokens, live, debugOverlay bool, now time.Time, refreshInterval time.Duration) wbox {
	title := fmt.Sprintf("%s %s", watchBoxSymbol(agent.AgentID), agent.Name)

	if !agent.Installed {
		return wbox{title: title, lines: []string{ansiDimGrey + "not installed\x1b[0m"}, width: width}
	}
	if !agent.Authenticated {
		return wbox{title: title, lines: []string{ansiDimGrey + "installed, not logged in\x1b[0m"}, width: width}
	}

	// stale drives issue 107's dimming of this panel's quota values, plus a
	// plain-text "· stale" complement on the "updated" caption below --
	// per-agent panels (unlike the compact All Usage table) have room to
	// spare for it (see the ticket's width-budget assessment).
	stale := agent.IsValueStale()

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

	// overlayLabel applies issue 131's debug-overlay countdown gauge to a
	// row label when debugOverlay is on, otherwise returns label unchanged.
	// It's a no-op when debugOverlay is false so normal-mode rendering is
	// byte-for-byte unaffected.
	overlayLabel := func(label string) string {
		if !debugOverlay {
			return label
		}
		return freshnessOverlayLabelForInterval(label, agent.LastRefreshed, now, refreshInterval)
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
			line := formatCompactGroupLine(overlayLabel(label), mg.Windows, contentW)
			if stale {
				line = staleValueANSI(line)
			}
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
			line := formatCompactGroupLine(overlayLabel(label), wins, contentW)
			if stale {
				line = staleValueANSI(line)
			}
			lines = append(lines, line)
		} else if len(wins) == 1 {
			line := formatCompactGroupLine(overlayLabel(wins[0].Name), wins, contentW)
			if stale {
				line = staleValueANSI(line)
			}
			lines = append(lines, line)
		}
	}

	// A fetch error is only worth a line when it actually explains missing
	// quota windows — an agent with model-group windows already showing has
	// nothing to apologize for.
	if agent.QuotaFetchError != "" && len(agent.ModelGroups) == 0 && agent.Session == nil && agent.Weekly == nil {
		lines = append(lines, fmt.Sprintf("%squota: unavailable (%s)\x1b[0m", ansiDimGrey, agent.QuotaFetchError))
	}

	if showTokens && agent.Tokens != nil {
		if live {
			spark := rate.Spark
			if spark == "" {
				spark = "warming up"
			}
			lines = append(lines, fmt.Sprintf("%s[T]\x1b[0m tok: %s total · %.0f/min [%s]",
				ansiBold, FormatNumber(agent.Tokens.TotalTokens), rate.PerMinute, spark))
		} else {
			lines = append(lines, fmt.Sprintf("%s[T]\x1b[0m tok: %s total",
				ansiBold, FormatNumber(agent.Tokens.TotalTokens)))
		}
	}

	if !agent.LastRefreshed.IsZero() {
		updated := "updated " + FormatAgo(agent.LastRefreshed)
		if stale {
			updated += " · stale"
		}
		lines = append(lines, ansiDimGrey+updated+"\x1b[0m")
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
		barOpts := watchUsageBarOptions(w.UsedPercent)
		barOpts.Width = compactBarWidth
		bar := rograph.RenderBar(w.UsedPercent, barOpts)
		line := fmt.Sprintf("%s %s %s%s", lbl, bar, padWatchUsagePercent(w.UsedPercent, 3), resetStr)
		if visLen(line) > contentW {
			line = fmt.Sprintf("%s %s %s", lbl, bar, padWatchUsagePercent(w.UsedPercent, 3))
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

	d1 := compactDurationText(w1)
	d2 := compactDurationText(w2)

	lbl := rograph.PadLabel(label, labelWidth)
	b1opts := watchUsageBarOptions(w1.UsedPercent)
	b1opts.Width = compactBarWidth
	b2opts := watchUsageBarOptions(w2.UsedPercent)
	b2opts.Width = compactBarWidth
	b1 := rograph.RenderBar(w1.UsedPercent, b1opts)
	b2 := rograph.RenderBar(w2.UsedPercent, b2opts)

	midStr := padWatchUsagePercentWithDuration(w1.UsedPercent, d1, 10)
	endStr := watchUsagePercentWithDuration(w2.UsedPercent, d2)

	// Format: Label [b1] pct1 d1 [b2] pct2 d2 (e.g. Gemini [███░] 91% 2h [░░░░] 0% 3d)
	line := fmt.Sprintf("%s %s %s %s %s", lbl, b1, midStr, b2, endStr)
	if visLen(line) <= contentW {
		return line
	}

	// Drop d2 if too long
	endStrNoD2 := watchUsagePercent(w2.UsedPercent)
	line = fmt.Sprintf("%s %s %s %s %s", lbl, b1, midStr, b2, endStrNoD2)
	if visLen(line) <= contentW {
		return line
	}

	// Drop d1 as well
	midStrNoD1 := padWatchUsagePercentWithDuration(w1.UsedPercent, "", 5)
	line = fmt.Sprintf("%s %s %s %s %s", lbl, b1, midStrNoD1, b2, endStrNoD2)
	if visLen(line) <= contentW {
		return line
	}

	// If still too long in very narrow box, shrink label
	for lw := labelWidth - 1; lw >= 6; lw-- {
		lblShrunk := rograph.PadLabel(label, lw)
		line = fmt.Sprintf("%s %s %s %s %s", lblShrunk, b1, watchUsagePercent(w1.UsedPercent), b2, watchUsagePercent(w2.UsedPercent))
		if visLen(line) <= contentW {
			return line
		}
	}

	// If still too long, shrink label to fit
	return line
}

// WatchOptions bundles optional customization for watch frame rendering.
type WatchOptions struct {
	Host          string
	ProcCounts    *AgentProcessCount
	Compact       bool
	ShowProcesses bool
	// ShowMic forces the Mic box on at startup (issue 244's `--mic` flag),
	// the same "explicit request wins over the current preset" role
	// ShowProcesses plays for Processes.
	ShowMic bool
	// MicLiveLevel/MicLiveAvailable (issue 245) carry this frame's live
	// peak/RMS reading from the watch loop's background micLiveManager,
	// resolved outside buildWatchFrameAt exactly like RemoteLoadSnapshot
	// below — the capture subprocess is a long-lived, once-per-session
	// resource (see RunWatchWithOptions), not something a pure frame-build
	// function should spawn itself.
	MicLiveLevel     float64
	MicLiveAvailable bool
	// ShowControls draws the [?]-triggered Controls overlay (issue 094)
	// instead of the normal panel grid for this frame.
	ShowControls bool

	// RemoteLoadHost is load.watch_host (issue 110), independent of Host —
	// when non-empty, a separate "[R] Remote Load (@RemoteLoadHost)" panel
	// is shown alongside the existing local [L] Load box. Empty means no
	// such panel at all (today's behavior, unchanged).
	RemoteLoadHost string
	// RemoteLoadSnapshot is the last known CPU/GPU reading for
	// RemoteLoadHost — from the streaming channel in --watch, or one plain
	// CollectRemote batch call in --summary/plain mode (Decision §2). Nil
	// renders the same "remote load unavailable" placeholder buildLoadBox
	// already uses for a stale/missing --host snapshot.
	RemoteLoadSnapshot *LoadSnapshot
	// RemoteLoadStreaming reports whether RemoteLoadSnapshot is currently
	// arriving over the --watch-only streaming channel (issue 110 §2) as
	// opposed to a plain batch poll — either the fallback path while
	// streaming is down, or the only path --summary/plain ever use. Always
	// false outside --watch. Purely a UI label; it never changes how the
	// data itself is fetched.
	RemoteLoadStreaming bool

	// DebugOverlay is issue 131's `!`-toggled per-agent freshness countdown
	// overlay: when true, each agent box's row label has its last 3
	// characters replaced with a countdown gauge instead of its normal
	// text. Display-only, in-process, never persisted.
	DebugOverlay bool
}

// controlsOverlayLines renders the full in-TUI Controls reference for
// `harnez usage --watch` (issue 094): every active keyboard command, grouped
// by purpose, so users don't have to reverse-engineer hidden box-title
// badges to discover what a key does. Direct panel toggles keep working
// (backward compat) and are documented here as secondary controls rather
// than promoted to the footer.
func controlsOverlayLines() []string {
	bold := func(s string) string { return ansiWrap("bold", s) }
	dim := func(s string) string { return ansiWrap("dim-grey", s) }
	wa := mustWatchActions()
	sym := wa.symbol

	var l []string
	l = append(l, bold("Controls")+"  "+dim("(press ?, Esc, q, or Enter to close)"))
	l = append(l, "")
	l = append(l, bold("View modes / presets"))
	l = append(l, fmt.Sprintf("  [%s]  cycle mode: default -> compact -> agents-only -> default", sym("cycle_preset")))
	l = append(l, fmt.Sprintf("  [%s]  reset panels to the default set", sym("reset_default")))
	l = append(l, "")
	l = append(l, bold("Panels (numbered toggles, btop-style)"))
	l = append(l, fmt.Sprintf("  [%s]  All Usage       [%s]  Claude", sym("toggle_all_usage"), sym("toggle_claude")))
	l = append(l, fmt.Sprintf("  [%s]  AGY             [%s]  Codex", sym("toggle_agy"), sym("toggle_codex")))
	l = append(l, fmt.Sprintf("  [%s]  History         [%s]  Processes", sym("toggle_history"), sym("toggle_processes")))
	l = append(l, fmt.Sprintf("  [%s]  Load            [%s]  Mic", sym("toggle_load"), sym("toggle_mic")))
	l = append(l, "")
	l = append(l, bold("Data rows"))
	l = append(l, fmt.Sprintf("  [%s]  token velocity / details on agent panels", sym("toggle_tokens")))
	l = append(l, "")
	l = append(l, bold("Session"))
	l = append(l, fmt.Sprintf("  [%s]              toggle remote/local host (when a host is configured)", sym("toggle_remote")))
	l = append(l, fmt.Sprintf("  [%s]              toggle per-agent freshness countdown gauge (debug overlay)", sym("toggle_debug_overlay")))
	l = append(l, fmt.Sprintf("  [%s] / Ctrl-C / Esc   quit", sym("quit")))
	l = append(l, "")
	l = append(l, dim("Numbered box toggles replaced the old C/G/O/H/P/L letter keys (issue"))
	l = append(l, dim("132); [a] keeps working as a compat alias for All Usage's [1]. The"))
	l = append(l, dim("footer only shows the most common controls — this overlay is the"))
	l = append(l, dim("full reference, sourced from spec/actions.yaml."))
	return l
}

func initialWatchSections(opts WatchOptions) watchSections {
	var sec watchSections
	if opts.Compact {
		sec = compactWatchSections()
	} else {
		sec = defaultWatchSections()
	}
	// --proc is an explicit request for the Processes panel and must win
	// regardless of --compact: without this, "--watch --compact --proc"
	// silently dropped the panel because compactWatchSections() never sets
	// Processes, and the early return above skipped the ShowProcesses check
	// entirely (issue 093).
	if opts.ShowProcesses {
		sec.Processes = true
	}
	// --mic (issue 244) gets the same "explicit request wins at startup"
	// treatment as --proc above, but deliberately not the further
	// dispatchWatchKey/cycle_preset reapplication --proc also gets ([m]
	// re-forces Processes back on after switching presets) — a live
	// audio-device box is enough of a niche, opt-in addition that keeping
	// it out of the preset-cycle machinery isn't worth widening
	// dispatchWatchKey's signature (and every existing call site) for.
	if opts.ShowMic {
		sec.Mic = true
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
	return buildWatchFrameAt(summary, rates, interval, sec, cols, rows, live, homeDir, historyDir, time.Now(), opts...)
}

// buildWatchFrameAt is buildWatchFrame with an explicit redraw timestamp. It
// keeps production wall-clock behavior while allowing the real watch frame
// path to be tested at exact points in its fetch countdown.
func buildWatchFrameAt(summary UsageSummary, rates map[string]agentRate, interval time.Duration, sec watchSections, cols, rows int, live bool, homeDir, historyDir string, now time.Time, opts ...WatchOptions) screenFrame {
	var opt WatchOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	// A frame's freshness gauges all use this redraw time. draw() builds a new
	// frame every second, so the values are recalculated without a usage fetch.

	if opt.ShowControls {
		ocols := cols - safetyMargin
		if ocols > maxTotalWidth {
			ocols = maxTotalWidth
		}
		if ocols < minTerminalWidth {
			ocols = minTerminalWidth
		}
		return fit(controlsOverlayLines(), ocols, rows)
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
	historyStatStr := fmt.Sprintf("   %shistory: %d %s (%s)\x1b[0m", ansiDimGrey, fileCount, fileWord, FormatBytes(totalBytes))

	// discovered is every agent the collector actually found real local/
	// remote state for (issue 083: self-hiding, auto-discovery display) — an
	// agent that isn't installed or configured on this machine never enters
	// the toggle/visibility machinery below at all, so it can't show up as
	// an empty box or a "hidden: [x]" toggle hint either.
	var discovered []AgentUsage
	for _, agent := range summary.Agents {
		// 7+ day stale agents auto-hide (issue 101), same gate as RenderText:
		// HasUsageData() alone can't tell "genuinely abandoned" apart from
		// "just refreshed a while ago."
		if agent.HasUsageData() && !agent.IsStale(DefaultDisplayStaleness) {
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

	// hiddenCount is a single summary of how many top-level boxes the user
	// (or the current mode/preset) has toggled off, replacing the old
	// per-box "[C] [H] [P]" badge list (issue 132): the controls overlay
	// (issue 094, opened via [?]) remains the place to see exactly which
	// boxes are hidden and why.
	hiddenCount := 0
	for _, agent := range discovered {
		if !sec.agentVisible(agent.AgentID) {
			hiddenCount++
		}
	}
	if !sec.History {
		hiddenCount++
	}
	if !sec.Processes {
		hiddenCount++
	}
	if !sec.Load {
		hiddenCount++
	}
	if !sec.AllUsage {
		hiddenCount++
	}
	hiddenHint := ""
	if hiddenCount > 0 {
		hiddenHint = fmt.Sprintf("   %s%d hidden (press ? for controls)\x1b[0m", ansiDimGrey, hiddenCount)
	}

	titlePrefix := "Agentic usage"
	if opt.Host != "" {
		titlePrefix = fmt.Sprintf("Agentic usage (@%s)", opt.Host)
	}

	header := []string{
		fmt.Sprintf("%s%s\x1b[0m  %s%s%s",
			ansiBold, titlePrefix, summary.Timestamp.Format("15:04:05 MST"), historyStatStr, hiddenHint),
		"",
	}
	var footer []string
	if live {
		wa := mustWatchActions()
		footer = []string{
			"",
			fmt.Sprintf("refresh every %s   %s[%s]controls  [%s]ode  [%s]emote  [%s]uit\x1b[0m",
				interval, ansiDimGrey, wa.symbol("toggle_controls"), wa.symbol("cycle_preset"), wa.symbol("toggle_remote"), wa.symbol("quit")),
		}
	}

	// Budget for the body: everything the header and footer already claim,
	// plus one spare line for the "panels dropped" note.
	budget := rows - len(header) - len(footer)
	if budget < 1 {
		budget = 1
	}

	var body []string
	// No agent had any real recorded usage found on this machine — say so
	// explicitly rather than silently rendering a screen with no agent
	// boxes at all (issue 083).
	if len(discovered) == 0 && len(summary.Agents) > 0 {
		body = append(body,
			ansiDimGrey+"no agent usage detected — install/configure Claude Code, Codex, or AGY\x1b[0m",
			ansiDimGrey+"or run `harnez agent-collector --once` to collect a fresh snapshot\x1b[0m",
			"",
		)
	}

	// Resolve data that a panel's build func needs but that must only be
	// fetched once per frame, even though each panel gets built twice below
	// (once to measure its natural content width, once at its final planned
	// width): a live CPU/GPU probe or a process scan must not run twice per
	// redraw just because the layout planner asked twice.
	var loadSnapshot *LoadSnapshot
	if sec.Load {
		switch {
		case summary.Load != nil:
			loadSnapshot = summary.Load
		case opt.Host == "":
			snap := LoadSnapshot{CPU: CurrentCPULoad(), GPUs: CurrentGPUs()}
			loadSnapshot = &snap
		}
	}
	// The Mic box is local-only (issue 244): a remote host's audio device
	// isn't observable over the existing --host snapshot machinery, so it
	// is never built in remote mode, matching Processes'
	// opt.Host == "" gate on loadSnapshot above.
	var micStatus MicStatus
	if sec.Mic && opt.Host == "" {
		micStatus = CurrentMicStatus()
		micStatus.LiveLevel = opt.MicLiveLevel
		micStatus.LiveAvailable = opt.MicLiveAvailable
	}
	var procCounts *AgentProcessCount
	if sec.Processes {
		procCounts = opt.ProcCounts
		if procCounts == nil {
			c := CountRunningAgentProcesses()
			procCounts = &c
		}
	}

	// panels lists every currently-enabled box in display order. Each
	// build func renders that panel's content at a given width — reused
	// both to measure the panel's natural (untruncated) content width and,
	// once uix.Layout has planned rows/widths, to render its final content
	// with that width's own graceful degradation (e.g. dropping a duration
	// label before falling back to renderWBox's ellipsis truncation).
	type panel struct {
		key   string
		build func(width int) wbox
	}
	var panels []panel
	if sec.AllUsage {
		panels = append(panels, panel{"a", func(w int) wbox { return buildAllUsageBoxAt(summary, w, opt.DebugOverlay, now, interval) }})
	}
	for _, agent := range visible {
		agent := agent
		panels = append(panels, panel{agentKey(agent.AgentID), func(w int) wbox {
			return buildAgentBoxAt(agent, rates[agent.AgentID], w, sec.Tokens, live, opt.DebugOverlay, now, interval)
		}})
	}
	if sec.History {
		panels = append(panels, panel{"H", func(w int) wbox { return buildHistoryBox(homeDir, historyDir, w) }})
	}
	if sec.Processes {
		panels = append(panels, panel{"P", func(w int) wbox { return buildProcessesBox(w, procCounts) }})
	}
	if sec.Load {
		panels = append(panels, panel{"L", func(w int) wbox { return buildLoadBox(w, opt.Host, loadSnapshot) }})
	}
	// Only registered when a real audio interface was actually found
	// (Available) — per issue 244, the box hides itself entirely on a
	// machine with no usable pactl/amixer backend rather than rendering an
	// empty or error-y panel just because the user's toggle is on.
	if sec.Mic && micStatus.Available {
		panels = append(panels, panel{"M", func(w int) wbox { return buildMicBox(w, micStatus) }})
	}
	if opt.RemoteLoadHost != "" {
		remoteHost, remoteSnap, remoteStreaming := opt.RemoteLoadHost, opt.RemoteLoadSnapshot, opt.RemoteLoadStreaming
		panels = append(panels, panel{"R", func(w int) wbox { return buildRemoteLoadBox(w, remoteHost, remoteSnap, remoteStreaming) }})
	}

	if len(panels) == 0 {
		body = append(body, ansiDimGrey+"(all panels hidden)\x1b[0m")
		lines := append(append(header, body...), footer...)
		return fit(lines, usable, rows)
	}

	// Plan rows/widths once from each panel's measured natural content
	// width (issue 093): a box sized to its own content never stretches
	// across arbitrary screen space, and a wide panel like All Usage packs
	// only as many columns as it actually needs, leaving the rest of the
	// row for panels like Load instead of starving them.
	boxes := make([]uix.Box, len(panels))
	for i, p := range panels {
		measured := p.build(measureWidth)
		natural := visLen(measured.title) + 6
		for _, l := range measured.lines {
			if n := visLen(l) + 4; n > natural {
				natural = n
			}
		}
		pref := natural
		if pref < minBoxWidth {
			pref = minBoxWidth
		}
		if pref > maxPanelContentWidth {
			pref = maxPanelContentWidth
		}
		boxes[i] = uix.Box{
			ID:        p.key,
			Title:     measured.title,
			MinWidth:  minBoxWidth,
			PrefWidth: pref,
			MaxWidth:  maxPanelContentWidth,
			Order:     i,
			Enabled:   true,
		}
	}
	plan := uix.Layout(boxes, uix.Options{Width: usable, Gap: boxGap})

	dropped := 0
	var droppedKeys []string
	for _, row := range plan.Rows {
		var cols [][]string
		var keys []string
		for _, pb := range row.Boxes {
			p := panels[pb.Order]
			cols = append(cols, renderWBox(p.build(pb.Width)))
			keys = append(keys, "["+pb.ID+"]")
		}
		lines := combineRow(cols)
		if len(body)+len(lines) <= budget {
			body = append(body, lines...)
		} else {
			dropped += len(row.Boxes)
			droppedKeys = append(droppedKeys, keys...)
		}
	}

	if dropped > 0 {
		note := fmt.Sprintf("%s… %s hidden — terminal too short\x1b[0m", ansiDimGrey, strings.Join(droppedKeys, " "))
		if len(body) < budget {
			body = append(body, note)
		} else if len(body) > 0 {
			body[len(body)-1] = note
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
//
// opts is optional; when given, its Compact field selects the same reduced
// panel set --watch --compact uses (issue 102), and ShowProcesses forces the
// Processes panel on regardless of Compact, matching initialWatchSections.
func RenderSummary(ctx context.Context, homeDir string, client *http.Client, out io.Writer, showProcesses bool, opts ...WatchOptions) {
	summary := CollectAll(ctx, homeDir, client)
	cols, rows := terminalSize(out)
	opt := firstOpt(opts)
	opt.ShowProcesses = opt.ShowProcesses || showProcesses
	sec := initialWatchSections(opt)
	frame := buildWatchFrame(summary, nil, 0, sec, cols, rows, false, homeDir, "", opt)
	for _, l := range frame.lines {
		fmt.Fprintln(out, l+"\x1b[0m")
	}
}

// RenderSummaryRemote prints one static frame of the compact grid using remote host collection.
func RenderSummaryRemote(ctx context.Context, host string, out io.Writer, showProcesses bool, opts ...WatchOptions) {
	opt := firstOpt(opts)
	opt.ShowProcesses = opt.ShowProcesses || showProcesses
	summary, procs, _ := CollectRemote(ctx, host, opt.ShowProcesses)
	cols, rows := terminalSize(out)
	opt.Host = host
	opt.ProcCounts = procs
	sec := initialWatchSections(opt)
	frame := buildWatchFrame(summary, nil, 0, sec, cols, rows, false, "", "", opt)
	for _, l := range frame.lines {
		fmt.Fprintln(out, l+"\x1b[0m")
	}
}

// firstOpt returns the first WatchOptions in opts, or the zero value.
func firstOpt(opts []WatchOptions) WatchOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return WatchOptions{}
}

// Startup splash tuning (issue 164). splashSpinnerSequenceName reuses
// spec/indicators.yaml's existing "braille-classic-10" spinner rather than
// adding a new one -- it already fits a one-off "work in progress" glyph.
const (
	splashSpinnerSequenceName = "braille-classic-10"
	splashFrameInterval       = 90 * time.Millisecond
	splashBarSweepPeriod      = 1200 * time.Millisecond
	splashBarWidth            = 24
)

// splashSpinnerGlyph returns the animated spinner glyph for the startup
// splash, cycling through spec/indicators.yaml's "braille-classic-10"
// sequence on splashFrameInterval ticks.
func splashSpinnerGlyph(elapsed time.Duration) string {
	frames := namedSequenceFrames(splashSpinnerSequenceName, "spinner")
	idx := int(elapsed/splashFrameInterval) % len(frames)
	return frames[idx]
}

// splashBarCapPercent bounds the determinate splash bar (issue 168) below
// 100% until the fetch it is tracking actually completes, so the bar never
// visually "finishes" and then appears to stall while the dashboard is
// still waiting on real data.
const splashBarCapPercent = 95.0

// splashBarPercent computes the startup splash bar's fill percentage.
//
// When haveEstimate is true, it is determinate: pct = 100 * elapsed /
// estimate, capped at splashBarCapPercent, so the bar fills monotonically
// left-to-right and reaches ~95% around when the tracked fetch
// (CollectAll/CollectRemote) is expected to finish -- estimate comes from
// this project's own persisted fetch-duration history (fetchdurations.go),
// not a guess.
//
// When haveEstimate is false (a true cold start: no prior sample exists yet
// for this fetch kind), it falls back to the original issue-164 sweep --
// 0-100-0 over splashBarSweepPeriod -- reading as "indeterminate work in
// progress" rather than fabricating a determinate bar with no real basis.
func splashBarPercent(elapsed, estimate time.Duration, haveEstimate bool) float64 {
	if !haveEstimate || estimate <= 0 {
		return splashBarPercentSweep(elapsed)
	}
	pct := 100 * float64(elapsed) / float64(estimate)
	switch {
	case pct < 0:
		return 0
	case pct > splashBarCapPercent:
		return splashBarCapPercent
	default:
		return pct
	}
}

// splashBarPercentSweep sweeps 0-100-0 over splashBarSweepPeriod -- the
// original issue-164 indeterminate bar, kept as splashBarPercent's
// no-estimate-yet fallback.
func splashBarPercentSweep(elapsed time.Duration) float64 {
	phase := elapsed % splashBarSweepPeriod
	half := splashBarSweepPeriod / 2
	if phase < half {
		return 100 * float64(phase) / float64(half)
	}
	return 100 - 100*float64(phase-half)/float64(half)
}

// centerLine pads s with leading spaces so its visible content centers
// within width columns. Trailing padding is left to fit/paint's per-line
// erase-to-end-of-line.
func centerLine(s string, width int) string {
	if s == "" {
		return ""
	}
	pad := (width - visLen(s)) / 2
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

// splashStatusMinDisplay is the minimum time each queued FetchStage event
// stays on screen before splashStatusAdvance lets the next one replace it.
// Local sub-fetches (claude/agy/codex) routinely complete in well under this
// project's splashFrameInterval paint cadence, so without pacing the splash
// only ever shows whichever event happened to be latest when the fetch
// finished -- the whole point of issue 169's status line (seeing each stage
// go by) was otherwise lost. 300ms is long enough to read a short line, short
// enough that a handful of queued events don't meaningfully delay handing
// control to the real dashboard once the fetch is actually done.
const splashStatusMinDisplay = 300 * time.Millisecond

// splashStatusEvent is one FetchStage transition queued for display.
type splashStatusEvent struct {
	source string
	stage  FetchStage
}

// splashBadge represents one completed (or failed) probe/collector badge (issue 252).
type splashBadge struct {
	source string
	ok     bool
}

// splashStatusState is the splash status line's queue of pending events plus
// which one is currently on screen. reportFetchStage (in RunWatchWithOptions)
// appends to queue; splashStatusAdvance pops from it on a minimum-display
// cadence so every event gets a turn instead of only the latest one.
type splashStatusState struct {
	queue   []splashStatusEvent
	current splashStatusEvent
	have    bool
	shownAt time.Time
	badges  []splashBadge
}

// splashStatusRecord records a new FetchStage event into st: appending the
// event to queue for the rolling single-line status log, and recording a
// completed/failed badge into st.badges when the event is terminal (FetchDone
// or FetchFailed).
func splashStatusRecord(st splashStatusState, source string, stage FetchStage) splashStatusState {
	if source == "" {
		return st
	}
	st.queue = append(st.queue, splashStatusEvent{source: source, stage: stage})
	if stage == FetchDone || stage == FetchFailed {
		ok := stage == FetchDone
		found := false
		for i, b := range st.badges {
			if b.source == source {
				st.badges[i].ok = ok
				found = true
				break
			}
		}
		if !found {
			st.badges = append(st.badges, splashBadge{source: source, ok: ok})
		}
	}
	return st
}

// splashStatusAdvance pops the next queued event into st.current once
// splashStatusMinDisplay has elapsed since the current one was shown (or
// immediately, if nothing has been shown yet). Pure function of (st, now) so
// it can be unit-tested without a real clock/goroutines.
func splashStatusAdvance(st splashStatusState, now time.Time, minDisplay time.Duration) splashStatusState {
	if len(st.queue) == 0 {
		return st
	}
	if st.have && now.Sub(st.shownAt) < minDisplay {
		return st
	}
	st.current = st.queue[0]
	st.queue = st.queue[1:]
	st.have = true
	st.shownAt = now
	return st
}

// splashStatusDrained reports whether the status queue is empty and the
// currently-shown event (if any) has been up long enough that advancing
// again would be a no-op -- i.e. there is nothing left worth waiting to
// display. RunWatchWithOptions uses this to decide when it's safe to leave
// the splash after the underlying fetch has finished: without it, a fetch
// that completes faster than its own queued events can be paced through
// would cut the display short exactly as before this fix.
func splashStatusDrained(st splashStatusState, now time.Time, minDisplay time.Duration) bool {
	return len(st.queue) == 0 && (!st.have || now.Sub(st.shownAt) >= minDisplay)
}

// splashStatusLine renders one FetchStage event (issue 169) as the single
// status line shown under the startup splash's bar, reporting which
// per-source fetch CollectAll/CollectRemote most recently started, finished,
// or failed. have is false before the first event has arrived (e.g. the
// whole fetch is being served from a fresh cache with no live sub-fetch at
// all), in which case the caller renders no status line.
func splashStatusLine(source string, stage FetchStage, have bool) string {
	if !have || source == "" {
		return ""
	}
	switch stage {
	case FetchStarted:
		return fmt.Sprintf("fetching %s...", source)
	case FetchDone:
		return fmt.Sprintf("%s done", source)
	case FetchFailed:
		return fmt.Sprintf("%s failed", source)
	default:
		return ""
	}
}

// splashSourceGlyph maps a probe source name to its brand/source glyph (issue 254).
func splashSourceGlyph(source string) string {
	switch source {
	case "agy":
		return "Λ"
	case "claude":
		return "✳"
	case "codex":
		return "֍"
	case "gemini":
		return "✦"
	case "mic":
		return "●"
	case "gpu", "load":
		return "⚙"
	default:
		return "●"
	}
}

// splashBadgesLine renders the cumulative list of completed/failed source badges
// (issues 252, 254) shown under the single rolling fetch status log line on the
// startup splash screen. Each source renders with its brand-specific glyph
// (e.g. "Λ agy", "✳ claude"), styled in chart-green on success or chart-warm
// on failure, with the source label in dim grey. Returns empty string when no
// badges are present.
func splashBadgesLine(badges []splashBadge) string {
	if len(badges) == 0 {
		return ""
	}
	items := make([]string, len(badges))
	for i, b := range badges {
		glyph := splashSourceGlyph(b.source)
		var color string
		if b.ok {
			color = "chart-green"
		} else {
			color = "chart-warm"
		}
		items[i] = ansiWrap(color, glyph) + " " + ansiWrap("dim-grey", b.source)
	}
	return strings.Join(items, "  ")
}

// buildSplashFrame paints the startup splash (issue 164): a spinner, an
// indeterminate progress bar (rendered via internal/rograph, the same bar
// renderer the rest of the dashboard uses), and a short hint line, centered
// in the terminal. RunWatchWithOptions shows this immediately after entering
// the alt-screen instead of leaving it blank while the first live fetch
// (CollectAll/CollectRemote) is in flight.
//
// animate controls the spinner/bar motion: true while the splash is timing
// out waiting for the fetch, false once Esc has skipped the wait. At that
// point the frame freezes -- it deliberately does not read lastSummary/
// lastProcs/currentHost, which the still-running background fetch goroutine
// owns exclusively until it finishes and calls draw() itself; painting the
// live dashboard from two goroutines at once would race on those vars.
//
// estimate/haveEstimate (issue 168) are the persisted fetch-duration
// estimate for the fetch kind this splash is waiting on, loaded once by
// RunWatchWithOptions before the splash loop starts; they drive
// splashBarPercent's determinate-vs-sweep choice. See splashBarPercent for
// the calculation itself.
//
// statusText (issue 169) is the latest rendered FetchStage event (see
// splashStatusLine) -- e.g. "fetching codex..." then "codex done" -- shown as
// exactly one line under the bar, or omitted entirely when empty (no event
// has arrived yet). It deliberately carries only the single latest event,
// never a scrolling history, per the ticket's "single line" scope.
//
// badgesText (issues 252, 254) is the cumulative completed source badges line
// (see splashBadgesLine) -- e.g. "Λ agy  ✳ claude  ● mic" -- shown directly
// under the status line, or omitted entirely when empty.
func buildSplashFrame(cols, rows int, elapsed time.Duration, animate bool, estimate time.Duration, haveEstimate bool, statusText, badgesText string) screenFrame {
	spinner := splashSpinnerGlyph(elapsed)
	hint := "Esc to skip"
	if !animate {
		spinner = splashSpinnerGlyph(0)
		hint = "waiting for first update..."
	}

	barOpts := watchBarOptions()
	barOpts.Width = splashBarWidth
	pct := splashBarPercent(elapsed, estimate, haveEstimate)
	if !animate {
		pct = 0
	}
	bar := rograph.RenderBar(pct, barOpts)

	title := ansiWrap("bold", spinner+"  harnez usage")
	hintLine := ansiWrap("dim-grey", hint)

	content := []string{title, "", bar}
	if animate {
		if statusText != "" {
			content = append(content, "", ansiWrap("dim-grey", statusText))
		}
		if badgesText != "" {
			if statusText == "" {
				content = append(content, "")
			}
			content = append(content, badgesText)
		}
	}
	content = append(content, "", hintLine)
	top := (rows - len(content)) / 2
	if top < 0 {
		top = 0
	}

	lines := make([]string, 0, rows)
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	for _, line := range content {
		lines = append(lines, centerLine(line, cols))
	}
	return fit(lines, cols, rows)
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

// micActivityTracker dynamically gates high-frequency UI redraws during
// speech activity (issue 258) with spec-driven high-fps mode.
type micActivityTracker struct {
	mu          sync.Mutex
	lastActive  time.Time
	gracePeriod time.Duration
	mode        MicLiveHighFPSMode
}

func newMicActivityTracker(gracePeriod time.Duration, mode MicLiveHighFPSMode) *micActivityTracker {
	return &micActivityTracker{
		gracePeriod: gracePeriod,
		mode:        mode,
	}
}

// ShouldRedraw reports whether an incoming mic reading warrants a fast UI redraw.
// Evaluates against the configured HighFPSMode (auto, on, off).
func (t *micActivityTracker) ShouldRedraw(reading micLiveReading, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.mode {
	case MicLiveHighFPSOff:
		return false
	case MicLiveHighFPSOn:
		return true
	case MicLiveHighFPSAuto:
		fallthrough
	default:
		if reading.Level > 0 {
			t.lastActive = now
			return true
		}
		if !t.lastActive.IsZero() && now.Sub(t.lastActive) <= t.gracePeriod {
			return true
		}
		return false
	}
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
	overlayOpen := false
	presetIdx := 0
	debugOverlay := false
	if opts.Compact {
		presetIdx = 1
	}

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

	// remote Load streaming (issue 110): started here, alongside this
	// function's other lifecycle-owned resources (stty restore above, tty/
	// SIGWINCH/alt-screen-buffer below), and torn down via the same defer-
	// based cleanup path on every exit from this function, Ctrl-C included
	// — Decision §4's ControlMaster lifetime is 1:1 with this --watch
	// process. remoteStreamStop is read by the deferred cleanup below and
	// written by the manager goroutine each time it (re)establishes a
	// stream, so cleanup can synchronously tear down whichever ssh
	// ControlMaster/child happens to be live at the moment this function
	// returns, instead of trusting a background goroutine to get around to
	// it after the process may already be exiting.
	remoteLoadHost := strings.TrimSpace(opts.RemoteLoadHost)
	var remoteLoadMu sync.Mutex
	var lastRemoteLoad *LoadSnapshot
	var remoteLoadStreaming bool
	var remoteStreamStop func()
	if remoteLoadHost != "" {
		setRemoteStreamStop := func(f func()) {
			remoteLoadMu.Lock()
			remoteStreamStop = f
			remoteLoadMu.Unlock()
		}
		setRemoteLoadStreaming := func(streaming bool) {
			remoteLoadMu.Lock()
			remoteLoadStreaming = streaming
			remoteLoadMu.Unlock()
			requestRedraw()
		}
		defer func() {
			remoteLoadMu.Lock()
			stopFn := remoteStreamStop
			remoteLoadMu.Unlock()
			if stopFn != nil {
				stopFn()
			}
		}()
		go runRemoteLoadManager(sigCtx, remoteLoadHost, &remoteLoadMu, &lastRemoteLoad, requestRedraw, setRemoteStreamStop, setRemoteLoadStreaming)
	}

	// splashActive/splashSkip (issue 164) gate the tty-reader goroutine below
	// while the startup splash is up: Esc must only abort the splash wait,
	// not quit the app, which is a deliberate deviation from Esc's normal
	// dispatchWatchKey meaning. splashActive starts true and flips false
	// (RunWatchWithOptions, below) once the splash phase ends, permanently,
	// for the rest of this run -- from then on every key goes through the
	// unchanged dispatchWatchKey path exactly as before this ticket.
	var splashActive atomic.Bool
	splashActive.Store(true)
	splashSkip := make(chan struct{}, 1)

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
				if splashActive.Load() {
					eff := dispatchSplashKey(buf[0])
					if eff.quit {
						stop()
						return
					}
					if eff.skip {
						select {
						case splashSkip <- struct{}{}:
						default:
						}
					}
					continue
				}
				secLock.Lock()
				st := watchKeyState{
					sec:            sec,
					overlayOpen:    overlayOpen,
					presetIdx:      presetIdx,
					configuredHost: configuredHost,
					activeHost:     activeHost,
					debugOverlay:   debugOverlay,
				}
				newSt, eff := dispatchWatchKey(st, buf[0], opts.ShowProcesses)
				sec, overlayOpen, presetIdx, activeHost, debugOverlay = newSt.sec, newSt.overlayOpen, newSt.presetIdx, newSt.activeHost, newSt.debugOverlay

				if eff.quit {
					secLock.Unlock()
					stop()
					return
				}
				if eff.redraw {
					requestRedraw()
				}
				if eff.fetch {
					requestFetch()
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

	// outMu (issue 164) serializes writes to out during the startup splash,
	// the one window where two goroutines can legitimately want to paint at
	// once: the splash animation loop (this goroutine) and the background
	// renderFrame()'s own draw() call once the first fetch completes. Every
	// other draw() call for the rest of the run happens on this goroutine
	// alone, same as before this ticket, so the lock is uncontended there.
	var outMu sync.Mutex

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

	// micLiveMgr (issue 245) owns the background capture subprocess for the
	// Mic box's live peak/RMS reading. draw() is the only place buildWatchFrame
	// is invoked for the life of this loop (see its own comment above), and
	// every draw() call happens on this single goroutine, so micLiveMgr can
	// be a plain local var here with no extra locking — same reasoning as
	// lastSummary/lastRates above. Started lazily the first time the Mic box
	// is actually visible in local mode, stopped the moment it isn't
	// (toggled off, or a remote host takes over) so no capture subprocess
	// ever runs while the box is off (issue 245 requirement 4), and always
	// stopped via the deferred cleanup below so a quit mid-session can't
	// leave one running (Zero Zombie Guarantee).
	var micLiveMgr *micLiveManager
	defer func() {
		if micLiveMgr != nil {
			micLiveMgr.Stop()
		}
	}()

	draw := func() {
		secLock.Lock()
		activeSec := sec
		showControls := overlayOpen
		activeDebugOverlay := debugOverlay
		secLock.Unlock()

		var remoteSnap *LoadSnapshot
		var remoteStreaming bool
		if remoteLoadHost != "" {
			remoteLoadMu.Lock()
			remoteSnap = lastRemoteLoad
			remoteStreaming = remoteLoadStreaming
			remoteLoadMu.Unlock()
		}

		wantMicLive := activeSec.Mic && currentHost == ""
		switch {
		case wantMicLive && micLiveMgr == nil:
			spec := watchMicLiveSpec()
			tracker := newMicActivityTracker(micGracePeriod, spec.HighFPSMode())
			micLiveMgr = startMicLiveManager(sigCtx, func(r micLiveReading) {
				if tracker.ShouldRedraw(r, time.Now()) {
					requestRedraw()
				}
			})
		case !wantMicLive && micLiveMgr != nil:
			micLiveMgr.Stop()
			micLiveMgr = nil
		}
		var micLive micLiveReading
		if micLiveMgr != nil {
			micLive = micLiveMgr.Snapshot()
		}

		cols, rows := terminalSize(out)
		frame := buildWatchFrame(lastSummary, lastRates, interval, activeSec, cols, rows, true, homeDir, historyDir, WatchOptions{
			Host:                currentHost,
			ProcCounts:          lastProcs,
			ShowControls:        showControls,
			RemoteLoadHost:      remoteLoadHost,
			RemoteLoadSnapshot:  remoteSnap,
			RemoteLoadStreaming: remoteStreaming,
			DebugOverlay:        activeDebugOverlay,
			MicLiveLevel:        micLive.Level,
			MicLiveAvailable:    micLive.Available,
		})
		outMu.Lock()
		frame.paint(out)
		outMu.Unlock()
	}

	// splashStatus (issue 169) queues FetchStage events reported by
	// CollectAllProgress/CollectRemoteProgress below, so the splash loop's
	// paintSplash (which runs on its own goroutine/timer, not renderFrame's)
	// can pace them onto the splash's single status line via
	// splashStatusAdvance/splashStatusMinDisplay. It is written from
	// renderFrame's fetch goroutine and read/advanced from the splash loop,
	// so it is guarded by its own mutex rather than reusing outMu/secLock,
	// which guard unrelated state.
	var splashStatusMu sync.Mutex
	var splashStatus splashStatusState
	reportFetchStage := func(source string, stage FetchStage) {
		splashStatusMu.Lock()
		splashStatus = splashStatusRecord(splashStatus, source, stage)
		splashStatusMu.Unlock()
	}

	// fetchAndUpdate does renderFrame's live fetch and lastSummary/lastRates
	// update but stops short of draw(). Split out so the startup path below
	// can run the fetch in the background while the splash is still showing,
	// without the fetch's completion instantly overwriting the splash mid-
	// status-queue-drain (see splashLoop's fetchDone/drained handling) --
	// renderFrame (fetchAndUpdate+draw, used by every other call site) still
	// paints immediately, since only the very first startup fetch has a
	// splash to coordinate with.
	fetchAndUpdate := func() {
		secLock.Lock()
		targetHost := activeHost
		procRequested := sec.Processes
		secLock.Unlock()

		currentHost = targetHost
		fetchStart := time.Now()
		var fresh UsageSummary
		if targetHost != "" {
			var procRes *AgentProcessCount
			fresh, procRes, _ = CollectRemoteProgress(sigCtx, targetHost, procRequested, reportFetchStage)
			lastProcs = procRes
		} else {
			var micWg sync.WaitGroup
			micWg.Add(1)
			go func() {
				defer micWg.Done()
				reportFetchStage("mic", FetchStarted)
				_ = CurrentMicStatus()
				reportFetchStage("mic", FetchDone)
			}()
			fresh = CollectAllProgress(sigCtx, homeDir, client, reportFetchStage)
			micWg.Wait()
			lastProcs = nil
			if historyDir != "" {
				// Best-effort: a missed append shouldn't interrupt the dashboard.
				_ = AppendHistory(historyDir, fresh)
			}
		}
		// Feed this fetch's wall-clock duration into the persisted
		// fetch-duration estimate (issue 168) that drives the startup
		// splash's determinate bar on the *next* --watch run for this fetch
		// kind. Skipped on a canceled context (Ctrl-C mid-fetch) since that
		// duration reflects an aborted fetch, not a real completion time,
		// and would drag the rolling estimate down artificially.
		if sigCtx.Err() == nil {
			recordFetchDuration(homeDir, fetchDurationKindForHost(targetHost), time.Since(fetchStart))
		}

		lastSummary = applyStaleQuota(fresh, lastSummary)
		lastRates = tracker.update(lastSummary)
	}

	renderFrame := func() {
		fetchAndUpdate()
		draw()
	}

	// Startup splash (issue 164): the first fetch (CollectAll/CollectRemote)
	// used to leave the alt-screen blank for its whole duration. Run it in
	// the background via fetchAndUpdate (renderFrame minus its draw() -- see
	// above) and paint an animated splash in its place until it finishes, so
	// the terminal never sits empty. draw() itself is deliberately deferred
	// to the splash loop below (not called here) so a fetch that finishes
	// before the status queue has been fully paced through (issue 169) can't
	// have its draw() silently overwritten by a subsequent splash repaint --
	// see the splashLoop comment below. Esc aborts only the wait (splashSkip,
	// handled by the tty-reader goroutine above via dispatchSplashKey) -- it
	// freezes the splash rather than quitting, and the fetch keeps running
	// in its goroutine regardless. Only one of this goroutine and the splash
	// loop below ever touches lastSummary/lastProcs/currentHost at a time:
	// the goroutine owns them exclusively until firstFrameDone closes, then
	// the splash loop (and, after it exits, the rest of this function)
	// resumes exclusive ownership for the rest of the run.
	firstFrameDone := make(chan struct{})
	go func() {
		defer close(firstFrameDone)
		fetchAndUpdate()
	}()

	// splashEstimate/splashHaveEstimate (issue 168) are loaded once, before
	// the splash loop starts, for the fetch kind this first renderFrame()
	// call is about to target (configuredHost, same as activeHost at this
	// point -- no key has been handled yet, so it cannot have changed).
	// They stay fixed for the life of this one splash: the estimate only
	// needs to be roughly right, and re-reading the cache on every tick
	// would just be needless disk I/O for a value that won't have changed
	// mid-fetch anyway.
	splashEstimate, splashHaveEstimate := loadFetchDurationEstimate(homeDir, fetchDurationKindForHost(configuredHost))

	// paintSplash advances the status queue on splashStatusMinDisplay pacing,
	// paints the frame, and reports whether the queue is now drained (see
	// splashStatusDrained) so the splash loop below knows when it's safe to
	// stop waiting once the underlying fetch has finished.
	paintSplash := func(elapsed time.Duration, animate bool) (drained bool) {
		cols, rows := terminalSize(out)
		now := time.Now()
		splashStatusMu.Lock()
		splashStatus = splashStatusAdvance(splashStatus, now, splashStatusMinDisplay)
		statusText := splashStatusLine(splashStatus.current.source, splashStatus.current.stage, splashStatus.have)
		badgesText := splashBadgesLine(splashStatus.badges)
		drained = splashStatusDrained(splashStatus, now, splashStatusMinDisplay)
		splashStatusMu.Unlock()
		frame := buildSplashFrame(cols, rows, elapsed, animate, splashEstimate, splashHaveEstimate, statusText, badgesText)
		outMu.Lock()
		frame.paint(out)
		outMu.Unlock()
		return drained
	}

	splashStart := time.Now()
	splashTicker := time.NewTicker(splashFrameInterval)
	paintSplash(0, true)
	// fetchDone latches true once firstFrameDone fires; firstFrameDoneCh is
	// then nilled so the select below stops selecting an already-closed
	// channel every iteration. The loop keeps ticking/painting after that
	// until the status queue drains (splashStatusDrained), so a fetch that
	// completes faster than its own events can be paced through still lets
	// each one get its splashStatusMinDisplay turn instead of jumping
	// straight to whichever event happened to be latest.
	fetchDone := false
	firstFrameDoneCh := firstFrameDone
splashLoop:
	for {
		select {
		case <-sigCtx.Done():
			splashTicker.Stop()
			return nil
		case <-firstFrameDoneCh:
			fetchDone = true
			firstFrameDoneCh = nil
			// No queued events yet (e.g. the whole fetch was served from
			// cache with no live sub-fetch to report) -- exit immediately
			// rather than waiting for the next splashTicker.C tick.
			if paintSplash(time.Since(splashStart), true) {
				splashTicker.Stop()
				splashActive.Store(false)
				draw()
				break splashLoop
			}
		case <-splashSkip:
			splashTicker.Stop()
			splashActive.Store(false)
			paintSplash(time.Since(splashStart), false)
			// The fetch is already running in the goroutine above; wait for
			// it (or a quit) without repainting, since lastSummary et al.
			// are its exclusively-owned state until it finishes. Esc means
			// "stop waiting", so unlike the normal exit below, draw() fires
			// the instant the fetch is ready -- no queue-drain grace period.
			select {
			case <-firstFrameDone:
				draw()
			case <-sigCtx.Done():
				return nil
			}
			break splashLoop
		case <-splashTicker.C:
			drained := paintSplash(time.Since(splashStart), true)
			if fetchDone && drained {
				splashTicker.Stop()
				splashActive.Store(false)
				draw()
				break splashLoop
			}
		}
	}

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

// remoteLoadRetryInterval is both the fallback batch-polling cadence and
// the delay between attempts to (re-)establish the streaming channel, while
// `--watch`'s remote Load box has no live stream (issue 110 Decision §2/§3:
// this is all an internal implementation detail, no user-facing flag).
var remoteLoadRetryInterval = 10 * time.Second

// runRemoteLoadManager owns the full lifecycle of --watch's remote Load
// data source for one configured host: it keeps trying to establish the
// streaming channel (StartRemoteLoadStream), and whenever that's down —
// never established, or dropped mid-session — falls back to periodic batch
// polling (CollectRemoteLoadSnapshot) so the box still shows fresh data,
// retrying the stream in the background the whole time. It runs for the
// life of ctx (RunWatchWithOptions's sigCtx) and returns once ctx is done.
//
// setStop is called with the current stream's teardown func every time a
// stream is established (nil once it ends), so RunWatchWithOptions's own
// deferred cleanup can synchronously tear down whichever ssh
// ControlMaster/child happens to be live at the moment it returns, rather
// than trusting this goroutine to get there first (see the "no zombie"
// requirement in issue 110 — the caller, not just this manager, must be
// able to guarantee cleanup on every exit path).
//
// setStreaming reports the current data-source mode so the UI can show
// whether the box is populated live over the stream or via a batch-poll
// fallback (issue 110 follow-up) — it is purely a display signal and never
// influences the fetch logic itself.
func runRemoteLoadManager(ctx context.Context, host string, mu *sync.Mutex, last **LoadSnapshot, redraw func(), setStop func(func()), setStreaming func(bool)) {
	setLast := func(snap *LoadSnapshot) {
		mu.Lock()
		*last = snap
		mu.Unlock()
		redraw()
	}

	for ctx.Err() == nil {
		ch, stop, err := StartRemoteLoadStream(ctx, host)
		if err != nil {
			// Streaming unavailable right now (stale remote harnez binary
			// predating `load-stream`, network blip, etc.) — fall back to
			// one batch poll now and retry establishing the stream after
			// remoteLoadRetryInterval.
			setStreaming(false)
			pollOnce(ctx, host, setLast)
			waitOrDone(ctx, remoteLoadRetryInterval)
			continue
		}

		setStop(stop)
		setStreaming(true)
		drainRemoteLoadStream(ctx, ch, setLast)
		stop()
		setStop(nil)
		setStreaming(false)

		if ctx.Err() != nil {
			return
		}
		// The stream ended (remote load-stream process died, connection
		// dropped, etc.) rather than this manager shutting down — same
		// fallback-then-retry behavior as a failed initial connect.
		pollOnce(ctx, host, setLast)
		waitOrDone(ctx, remoteLoadRetryInterval)
	}
}

// drainRemoteLoadStream reads every sample off ch, applying each via
// setLast, until ch closes (the stream ended) or ctx is done.
func drainRemoteLoadStream(ctx context.Context, ch <-chan LoadSnapshot, setLast func(*LoadSnapshot)) {
	for {
		select {
		case snap, ok := <-ch:
			if !ok {
				return
			}
			s := snap
			setLast(&s)
		case <-ctx.Done():
			return
		}
	}
}

// pollOnce fetches one batch remote Load snapshot and, only on success,
// applies it via setLast — a failed poll leaves the last-known snapshot in
// place (stale-but-present) rather than blanking the box, matching how a
// failed --host fetch elsewhere in this file already degrades.
func pollOnce(ctx context.Context, host string, setLast func(*LoadSnapshot)) {
	snap, err := CollectRemoteLoadSnapshot(ctx, host)
	if err == nil && snap != nil {
		setLast(snap)
	}
}

// waitOrDone blocks for d or until ctx is done, whichever comes first.
func waitOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
