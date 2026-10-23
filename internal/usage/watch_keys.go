package usage

type namedWindow struct {
	label string
	w     QuotaWindow
}

// watchSections controls which panels the `--watch` grid currently shows.
// Toggled interactively via keypress; see RunWatch. The key for each panel
// (shown in its own title bar, btop-style) is fixed here.
type watchSections struct {
	Table      bool
	AllUsage   bool
	Processes  bool
	Load       bool
	Mic        bool
	Tokens     bool
	RemoteLoad bool
}

func defaultWatchSections() watchSections {
	return initialWatchSections(WatchOptions{Mode: "normal"})
}

func compactWatchSections() watchSections {
	return initialWatchSections(WatchOptions{Mode: "compact"})
}

// watchPresetOrder and watchPresetNames define the fixed cycle order for the
// [m] mode/preset shortcut (issue 094): table -> compact -> table. Cycling
// is keyed off an explicit index rather than sniffing sec's current field
// values, since sec can drift away from any named preset via individual
// panel toggles ([1]/[a]/etc.) between presses.
var watchPresetOrder = []watchSections{
	defaultWatchSections(),
	compactWatchSections(),
}

var watchPresetNames = []string{"table", "compact"}

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
	sec               watchSections
	overlayOpen       bool
	diagnosticsOpen   bool
	diagnosticsOffset int
	presetIdx         int
	configuredHost    string
	activeHost        string
	// debugOverlay is issue 131's `!`-toggled per-agent freshness countdown
	// overlay: in-process only, never persisted, independent of overlayOpen
	// (the [?] Controls reference overlay).
	debugOverlay bool
	debugBox     bool
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
//  3. [m]/[M] cycles the table -> compact -> table preset, re-applying
//     showProcesses so an explicit --proc request keeps winning across
//     preset changes the same way initialWatchSections already makes it
//     win at startup.
//  4. [!] toggles the debug overlay (issue 131's per-agent freshness
//     countdown gauge) — in-process only, never persisted.
//  5. Existing single-key panel toggles (applyWatchSectionKey) keep working
//     unchanged, for backward compatibility.
//  6. [r]/[R] flips remote/local when a host was configured; [q]/[Q]/Ctrl-C
//     quits. A lone Esc is ignored on the main view and closes overlays.
func dispatchWatchKey(st watchKeyState, key byte, showProcesses bool) (watchKeyState, watchKeyEffect) {
	action := mustWatchActions().actionForKey(key)

	switch {
	case action == "toggle_diagnostics":
		st.diagnosticsOpen = !st.diagnosticsOpen
		if st.diagnosticsOpen {
			st.diagnosticsOffset = 0
		}
		st.overlayOpen = false
		return st, watchKeyEffect{redraw: true}
	case action == "toggle_controls":
		st.diagnosticsOpen = false
		st.overlayOpen = !st.overlayOpen
		return st, watchKeyEffect{redraw: true}

	case st.diagnosticsOpen:
		switch key {
		case 'q', 'Q', 3, 27, '\r', '\n', 'l', 'L', '?':
			st.diagnosticsOpen = false
			return st, watchKeyEffect{redraw: true}
		case 'j', 'd':
			st.diagnosticsOffset++
			return st, watchKeyEffect{redraw: true}
		case 'k', 'u':
			if st.diagnosticsOffset > 0 {
				st.diagnosticsOffset--
			}
			return st, watchKeyEffect{redraw: true}
		}
		return st, watchKeyEffect{}

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
		st.debugBox = st.debugOverlay
		return st, watchKeyEffect{redraw: true}

	case action == "toggle_debug_box":
		st.debugBox = !st.debugBox
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
			st.sec.RemoteLoad = !st.sec.RemoteLoad
			return st, watchKeyEffect{redraw: true}
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
// dispatchWatchKey: byte 27 (Esc) here only skips the splash wait. Ctrl-C
// (byte 3) quits either way, since it is the universal interrupt. Every
// other key is ignored during splash -- the dashboard (sec/overlayOpen/etc.)
// is not showing yet, so panel toggles have nothing to act on.
func dispatchSplashKey(key byte) splashKeyEffect {
	switch key {
	case 3, 'q', 'Q':
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
