package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/uix"
)

// WatchOptions bundles optional customization for watch frame rendering.
type WatchOptions struct {
	Host string
	// StoredUsageLoader reads the most recent collector snapshot for local views.
	StoredUsageLoader func(context.Context) UsageSummary
	// Loading distinguishes the first interactive frame from an empty result.
	Loading    bool
	ProcCounts *AgentProcessCount
	// Mode selects one of the spec-defined usage views. Empty uses the spec default.
	Mode    string
	Compact bool
	// Minimal is retained for compatibility with direct WatchOptions callers;
	// CLI callers select the same mode through Mode.
	Minimal       bool
	ShowProcesses bool
	// Splash enables the startup splash screen during initial collection in --watch (issue 664).
	Splash bool
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
	// ShowDiagnostics draws the bounded in-session collector fetch log.
	ShowDiagnostics   bool
	Diagnostics       []string
	DiagnosticsOffset int

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
	// overlay: when true, each All Usage row label has its last 3
	// characters replaced with a countdown gauge instead of its normal
	// text. The usage table ignores it. Display-only, in-process, never
	// persisted.
	DebugOverlay bool
}

func compactUsageView(opts WatchOptions) bool {
	mode := usageViewModeName(opts.Mode, opts.Compact, opts.Minimal)
	return mode == "compact" || mode == "minimal"
}

func inheritUsageView(base, frame WatchOptions) WatchOptions {
	frame.Mode = base.Mode
	frame.Compact = base.Compact
	frame.Minimal = base.Minimal
	return frame
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
	l = append(l, fmt.Sprintf("  [%s]  cycle mode: table -> compact -> table", sym("cycle_preset")))
	l = append(l, fmt.Sprintf("  [%s]  reset panels to the default set", sym("reset_default")))
	l = append(l, "")
	l = append(l, bold("Panels (numbered toggles, btop-style)"))
	l = append(l, fmt.Sprintf("  [%s]  All Usage       [%s]  Processes", sym("toggle_all_usage"), sym("toggle_processes")))
	l = append(l, fmt.Sprintf("  [%s]  Load            [%s]  Mic", sym("toggle_load"), sym("toggle_mic")))
	l = append(l, "")
	l = append(l, bold("Data rows"))
	l = append(l, fmt.Sprintf("  [%s]  Tokens and Tok/min columns of the usage table", sym("toggle_tokens")))
	l = append(l, "")
	l = append(l, bold("Session"))
	l = append(l, fmt.Sprintf("  [%s]              toggle remote/local host (when a host is configured)", sym("toggle_remote")))
	l = append(l, fmt.Sprintf("  [%s]              open/close fetch diagnostics", sym("toggle_diagnostics")))
	l = append(l, fmt.Sprintf("  [%s]              toggle per-agent freshness countdown gauge (All Usage panel)", sym("toggle_debug_overlay")))
	l = append(l, fmt.Sprintf("  [%s/Q] / Ctrl-C   quit", sym("quit")))
	l = append(l, "")
	l = append(l, dim("[a] is a compat alias for All Usage's [1]. The footer only shows the"))
	l = append(l, dim("most common controls — this overlay is the full reference, sourced"))
	l = append(l, dim("from spec/actions.yaml."))
	return l
}

func initialWatchSections(opts WatchOptions) watchSections {
	mode := usageViewMode(opts.Mode, opts.Compact, opts.Minimal)
	var sec watchSections
	sec.Tokens = mode.tokenDetails()
	for _, panel := range mode.Panels {
		switch panel {
		case "table":
			sec.Table = true
		case "all_usage":
			sec.AllUsage = true
		case "processes":
			sec.Processes = true
		case "load":
			sec.Load = true
		case "mic":
			sec.Mic = true
		case "remote_load":
			sec.RemoteLoad = true
		}
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

func diagnosticsOverlayLines(entries []string, offsets ...int) []string {
	dim := func(s string) string { return ansiWrap("dim-grey", s) }
	lines := []string{ansiWrap("bold", "Fetch diagnostics") + "  " + dim("(press l, Esc, q, or ? to close)"), ""}
	if len(entries) == 0 {
		lines = append(lines, dim("no collector events yet"))
		return lines
	}
	offset := 0
	if len(offsets) > 0 && offsets[0] > 0 {
		offset = offsets[0]
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	lines = append(lines, entries[offset:]...)
	return lines
}

func formatDiagnosticEvent(source string, stage FetchStage, at time.Time) string {
	name := "unknown"
	switch stage {
	case FetchStarted:
		name = "started"
	case FetchDone:
		name = "done"
	case FetchFailed:
		name = "failed"
	}
	return fmt.Sprintf("%s %-6s %s", at.Format("15:04:05"), name, source)
}

func formatDetailedDiagnosticEvent(e FetchDiagnostic, at time.Time) string {
	if e.Stage == FetchStarted {
		return formatDiagnosticEvent(e.Source, e.Stage, at)
	}
	line := fmt.Sprintf("%s %-6s %s (%s)", at.Format("15:04:05"), diagnosticStageName(e.Stage), e.Source, e.Duration.Round(time.Millisecond))
	if e.Error != "" {
		line += ": " + strings.ReplaceAll(strings.ReplaceAll(e.Error, "\n", " "), "\r", " ")
	}
	return line
}

func diagnosticStageName(stage FetchStage) string {
	switch stage {
	case FetchDone:
		return "done"
	case FetchFailed:
		return "failed"
	default:
		return "unknown"
	}
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
func buildWatchFrame(summary UsageSummary, rates map[string]agentRate, interval time.Duration, sec watchSections, cols, rows int, live bool, opts ...WatchOptions) screenFrame {
	return buildWatchFrameAt(summary, rates, interval, sec, cols, rows, live, time.Now(), opts...)
}

// buildWatchFrameAt is buildWatchFrame with an explicit redraw timestamp. It
// keeps production wall-clock behavior while allowing the real watch frame
// path to be tested at exact points in its fetch countdown.
func buildWatchFrameAt(summary UsageSummary, rates map[string]agentRate, interval time.Duration, sec watchSections, cols, rows int, live bool, now time.Time, opts ...WatchOptions) screenFrame {
	var opt WatchOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	mode := usageViewMode(opt.Mode, opt.Compact, opt.Minimal)
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
	if opt.ShowDiagnostics {
		ocols := cols - safetyMargin
		if ocols > maxTotalWidth {
			ocols = maxTotalWidth
		}
		if ocols < minTerminalWidth {
			ocols = minTerminalWidth
		}
		return fit(diagnosticsOverlayLines(opt.Diagnostics, opt.DiagnosticsOffset), ocols, rows)
	}

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
		if agent.HasUsageData() && !agent.IsStale(DefaultDisplayStaleness()) {
			discovered = append(discovered, agent)
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
	if !sec.Processes {
		hiddenCount++
	}
	if !sec.Load {
		hiddenCount++
	}
	if !sec.AllUsage {
		hiddenCount++
	}
	if opt.RemoteLoadHost != "" && !sec.RemoteLoad {
		hiddenCount++
	}
	hiddenHint := ""
	if hiddenCount > 0 && mode.hiddenList() {
		hiddenHint = fmt.Sprintf("   %s%d hidden (press ? for controls)\x1b[0m", ansiDimGrey, hiddenCount)
	}

	titlePrefix := "Agentic usage"
	if opt.Host != "" {
		titlePrefix = fmt.Sprintf("Agentic usage (@%s)", opt.Host)
	}

	var header []string
	if mode.titleBar() {
		ageLabel := usageSnapshotAgeLabel(summary.Timestamp, now)
		header = []string{
			fmt.Sprintf("%s%s\x1b[0m  %s%s",
				ansiBold, titlePrefix, summary.Timestamp.Local().Format("15:04:05 MST")+" · "+ageLabel, hiddenHint),
			"",
		}
	}
	var footer []string
	if live && usageStatusFooterFits(rows, opt) {
		wa := mustWatchActions()
		footer = []string{
			"",
			fmt.Sprintf("refresh every %s   %s[%s]controls  [%s]ode  [%s]emote  [%s/Q]uit\x1b[0m",
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
	if opt.Loading {
		_, _, _, _, _, _ = usageDurations()
		body = append(body, ansiDimGrey+usageSpecLoadingLabel+"\x1b[0m", "")
	}
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
	// The normal view mode's plain usage table sits above any boxes.
	if sec.Table {
		if table := usageTableLines(discovered, rates, sec.Tokens, now); len(table) > 0 {
			body = append(body, table...)
			body = append(body, "")
		}
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
			// Interactive timelines grow on each tick. Cold burst sampling
			// deliberately sleeps ~2s and must not stall the UI's first frame.
			snap := LoadSnapshot{CPU: currentCPULoad(!live), GPUs: currentGPUs(!live)}
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
	if sec.RemoteLoad && opt.RemoteLoadHost != "" {
		remoteHost, remoteSnap, remoteStreaming := opt.RemoteLoadHost, opt.RemoteLoadSnapshot, opt.RemoteLoadStreaming
		panels = append(panels, panel{"R", func(w int) wbox { return buildRemoteLoadBox(w, remoteHost, remoteSnap, remoteStreaming) }})
	}

	if len(panels) == 0 && sec.Table {
		if len(body) > 0 && body[len(body)-1] == "" {
			body = body[:len(body)-1]
		}
		lines := append(append(header, body...), footer...)
		return fit(lines, usable, rows)
	}
	if len(panels) == 0 {
		message := "(all panels hidden)"
		if !mode.titleBar() && !mode.hiddenList() && !mode.overflowHint() {
			message = "(no visible panels)"
		}
		body = append(body, ansiDimGrey+message+"\x1b[0m")
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

	if dropped > 0 && mode.overflowHint() {
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

// RenderSummary prints one static frame of the spec-selected btop-style grid
// used by `--watch`, then returns — no alternate screen, polling loop, or
// keyboard handling.
//
// opts selects the same view as --watch. The static frame reserves the
// watch status-bar rows so constrained terminals show the same panel detail.
func RenderSummary(ctx context.Context, homeDir string, client *http.Client, out io.Writer, showProcesses bool, opts ...WatchOptions) {
	opt := firstOpt(opts)
	var summary UsageSummary
	if opt.StoredUsageLoader != nil {
		summary = opt.StoredUsageLoader(ctx)
	} else {
		summary = storedSummaryOrError(ctx)
	}
	cols, rows := terminalSize(out)
	opt.ShowProcesses = opt.ShowProcesses || showProcesses
	rows = staticUsageViewRows(rows, opt)
	sec := initialWatchSections(opt)
	frame := buildWatchFrame(summary, nil, 0, sec, cols, rows, false, opt)
	for _, l := range frame.lines {
		fmt.Fprintln(out, l+"\x1b[0m")
	}
}

// RenderSummaryWithUsage renders one static frame from an already-collected
// summary, allowing compact callers to query the store first.
func RenderSummaryWithUsage(summary UsageSummary, out io.Writer, showProcesses bool, opts ...WatchOptions) {
	opt := firstOpt(opts)
	cols, rows := terminalSize(out)
	renderSummaryWithUsageAt(summary, out, showProcesses, opt, cols, rows)
}

func renderSummaryWithUsageAt(summary UsageSummary, out io.Writer, showProcesses bool, opt WatchOptions, cols, rows int) {
	opt.ShowProcesses = opt.ShowProcesses || showProcesses
	rows = staticUsageViewRows(rows, opt)
	frame := buildWatchFrame(summary, nil, 0, initialWatchSections(opt), cols, rows, false, opt)
	for _, line := range frame.lines {
		fmt.Fprintln(out, line+"\x1b[0m")
	}
}

func staticUsageViewRows(rows int, opts WatchOptions) int {
	if usageStatusFooterFits(rows, opts) {
		rows -= 2 // the interactive footer is two rows: a blank and the controls line.
	}
	if rows < 1 {
		return 1
	}
	return rows
}

// usageStatusFooterFits leaves room for the header, at least one body line,
// and the two-line interactive footer. Static rendering reserves those same
// rows so one-shot and watch retain identical panel and overflow detail.
func usageStatusFooterFits(rows int, opts WatchOptions) bool {
	mode := usageViewMode(opts.Mode, opts.Compact, opts.Minimal)
	if !mode.statusBar() {
		return false
	}
	headerRows := 0
	if mode.titleBar() {
		headerRows = 2
	}
	return rows >= headerRows+3
}

// RenderSummaryRemote prints one static frame of the spec-selected grid using remote host collection.
func RenderSummaryRemote(ctx context.Context, host string, out io.Writer, showProcesses bool, opts ...WatchOptions) {
	opt := firstOpt(opts)
	opt.ShowProcesses = opt.ShowProcesses || showProcesses
	summary, procs, _ := CollectRemote(ctx, host, opt.ShowProcesses)
	cols, rows := terminalSize(out)
	rows = staticUsageViewRows(rows, opt)
	opt.Host = host
	opt.ProcCounts = procs
	sec := initialWatchSections(opt)
	frame := buildWatchFrame(summary, nil, 0, sec, cols, rows, false, opt)
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
