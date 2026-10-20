package usage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
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

const watchEscapeSequenceTimeout = 50 * time.Millisecond

type watchReadDeadliner interface {
	SetReadDeadline(time.Time) error
}

// readWatchKey returns one key from the watch terminal. Escape-prefixed
// terminal sequences are consumed as a unit and represented by key 0 so
// their payload can never trigger a watch action. A lone Esc is returned as
// 27 after the short sequence window, allowing overlays to close promptly.
func readWatchKey(r io.Reader) (byte, error) {
	readByte := func() (byte, error) {
		var buf [1]byte
		_, err := io.ReadFull(r, buf[:])
		return buf[0], err
	}

	key, err := readByte()
	if err != nil || key != 27 {
		return key, err
	}

	deadliner, hasDeadline := r.(watchReadDeadliner)
	readContinuation := func() (byte, error) {
		if hasDeadline {
			if err := deadliner.SetReadDeadline(time.Now().Add(watchEscapeSequenceTimeout)); err != nil {
				return 0, err
			}
		}
		return readByte()
	}
	if hasDeadline {
		defer deadliner.SetReadDeadline(time.Time{})
	}

	first, err := readContinuation()
	if err != nil {
		if errorsIsEscapeEnd(err) {
			return 27, nil
		}
		return 0, err
	}
	switch first {
	case '[':
		sequence := make([]byte, 1, 16)
		sequence[0] = first
		for len(sequence) < 128 {
			b, readErr := readContinuation()
			if readErr != nil {
				if errorsIsEscapeEnd(readErr) {
					return 0, nil
				}
				return 0, readErr
			}
			sequence = append(sequence, b)
			if b >= 0x40 && b <= 0x7e {
				if string(sequence) == "[200~" {
					if err := consumeBracketedPaste(readContinuation); err != nil && !errorsIsEscapeEnd(err) {
						return 0, err
					}
				}
				return 0, nil
			}
		}
		return 0, nil
	case 'O': // SS3 function/navigation keys, for example F1 and keypad arrows.
		_, err := readContinuation()
		if errorsIsEscapeEnd(err) {
			return 0, nil
		}
		return 0, err
	case ']', 'P', '^', '_': // OSC, DCS, PM and APC string sequences.
		err := consumeStringSequence(readContinuation, first == ']')
		if errorsIsEscapeEnd(err) {
			return 0, nil
		}
		return 0, err
	default:
		if first == 3 || first == 'q' || first == 'Q' {
			return first, nil
		}
		return 0, nil
	}
}

func errorsIsEscapeEnd(err error) bool {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func consumeBracketedPaste(readByte func() (byte, error)) error {
	const end = "\x1b[201~"
	var tail []byte
	for {
		b, err := readByte()
		if err != nil {
			return err
		}
		tail = append(tail, b)
		if len(tail) > len(end) {
			tail = tail[1:]
		}
		if string(tail) == end {
			return nil
		}
	}
}

func consumeStringSequence(readByte func() (byte, error), osc bool) error {
	previousEsc := false
	for {
		b, err := readByte()
		if err != nil {
			return err
		}
		if osc && b == 7 { // OSC may end with BEL.
			return nil
		}
		if previousEsc && b == '\\' {
			return nil
		}
		previousEsc = b == 27
	}
}

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

// RunWatch redraws a compact, btop-style usage dashboard in place on a fixed
// interval, instead of the full `harnez usage` report which is too chatty to
// redraw every tick. It polls stored data at most once per interval.
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

// redrawThrottler coalesces and rate-limits redraw requests to enforce a
// maximum frame rate (issue 259: MinFrameInterval >= 50ms / <= 20 FPS cap).
// Background measurement hooks (e.g. mic audio PCM streams) can fire dozens
// of times per second without overwhelming the TUI renderer.
type redrawThrottler struct {
	mu          sync.Mutex
	minInterval time.Duration
	lastFired   time.Time
	pending     bool
	timer       *time.Timer
	trigger     func()
	stopped     bool
}

func newRedrawThrottler(minInterval time.Duration, trigger func()) *redrawThrottler {
	if minInterval < 50*time.Millisecond {
		minInterval = 50 * time.Millisecond
	}
	return &redrawThrottler{
		minInterval: minInterval,
		trigger:     trigger,
	}
}

func (t *redrawThrottler) Request() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	now := time.Now()
	elapsed := now.Sub(t.lastFired)
	if !t.pending && (t.lastFired.IsZero() || elapsed >= t.minInterval) {
		t.lastFired = now
		t.trigger()
		return
	}

	if t.pending {
		return
	}

	t.pending = true
	delay := t.minInterval - elapsed
	if delay < 0 {
		delay = 0
	}
	t.scheduleTimerLocked(delay)
}

func (t *redrawThrottler) scheduleTimerLocked(delay time.Duration) {
	t.timer = time.AfterFunc(delay, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.stopped || !t.pending {
			return
		}
		now := time.Now()
		elapsed := now.Sub(t.lastFired)
		if elapsed < t.minInterval {
			t.scheduleTimerLocked(t.minInterval - elapsed)
			return
		}
		t.pending = false
		t.lastFired = now
		t.trigger()
	})
}

func (t *redrawThrottler) MarkDrawn(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastFired = now
}

func (t *redrawThrottler) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	t.pending = false
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

func RunWatchWithOptions(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, historyDir string, opts WatchOptions) error {
	_ = historyDir // retained for source compatibility; watch no longer writes history.
	if interval < MinWatchInterval {
		interval = MinWatchInterval
	}

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Open nonblocking so Go's poller can interrupt Read when Close runs.
	// A blocking /dev/tty read can otherwise hold cleanup indefinitely.
	var oldState *term.State
	tty, ttyErr := os.OpenFile("/dev/tty", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	restoreTerminal := func() {
		if oldState != nil {
			_ = term.Restore(int(tty.Fd()), oldState)
			oldState = nil
		}
	}
	if ttyErr == nil {
		var err error
		oldState, err = term.MakeRaw(int(tty.Fd()))
		if err != nil {
			tty.Close()
			return fmt.Errorf("watch terminal: %w", err)
		}
		defer func() {
			restoreTerminal()
			tty.Close()
		}()
	}

	var secLock sync.Mutex
	sec := initialWatchSections(opts)
	overlayOpen := false
	diagnosticsOpen := false
	diagnosticsOffset := 0
	presetIdx := 0
	debugOverlay := false
	if compactUsageView(opts) {
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
	requestRedrawRaw := func() {
		select {
		case redrawChan <- struct{}{}:
		default:
		}
	}

	spec := watchMicLiveSpec()
	throttler := newRedrawThrottler(spec.HighFPSDelay(), requestRedrawRaw)
	defer throttler.Stop()

	requestRedraw := func() {
		throttler.Request()
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
		remoteDone := make(chan struct{})
		defer func() {
			stop()
			<-remoteDone
			remoteLoadMu.Lock()
			stopFn := remoteStreamStop
			remoteLoadMu.Unlock()
			if stopFn != nil {
				stopFn()
			}
		}()
		go func() {
			defer close(remoteDone)
			runRemoteLoadManager(sigCtx, remoteLoadHost, &remoteLoadMu, &lastRemoteLoad, requestRedraw, setRemoteStreamStop, setRemoteLoadStreaming)
		}()
	}

	var splashActive atomic.Bool
	splashActive.Store(opts.Splash)
	splashSkip := make(chan struct{}, 1)

	if ttyErr == nil {
		readerDone := make(chan struct{})
		defer func() {
			stop()
			restoreTerminal()
			tty.Close()
			<-readerDone
		}()
		go func() {
			defer close(readerDone)
			for {
				key, err := readWatchKey(tty)
				if err != nil {
					stop()
					return
				}
				if splashActive.Load() {
					eff := dispatchSplashKey(key)
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
					sec:               sec,
					overlayOpen:       overlayOpen,
					diagnosticsOpen:   diagnosticsOpen,
					diagnosticsOffset: diagnosticsOffset,
					presetIdx:         presetIdx,
					configuredHost:    configuredHost,
					activeHost:        activeHost,
					debugOverlay:      debugOverlay,
				}
				newSt, eff := dispatchWatchKey(st, key, opts.ShowProcesses)
				sec, overlayOpen, diagnosticsOpen, presetIdx, activeHost, debugOverlay, diagnosticsOffset = newSt.sec, newSt.overlayOpen, newSt.diagnosticsOpen, newSt.presetIdx, newSt.activeHost, newSt.debugOverlay, newSt.diagnosticsOffset

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
	winchDone := make(chan struct{})
	defer func() { stop(); <-winchDone }()
	go func() {
		defer close(winchDone)
		for {
			select {
			case <-sigCtx.Done():
				return
			case <-winch:
				requestRedraw()
			}
		}
	}()

	tracker := newRateTracker()

	lastSummary := UsageSummary{Timestamp: time.Now()}
	loading := true
	var lastRates map[string]agentRate
	var lastProcs *AgentProcessCount
	currentHost := configuredHost
	var diagnosticsMu sync.Mutex
	var diagnostics []string
	appendDiagnostic := func(source string, stage FetchStage) {
		diagnosticsMu.Lock()
		defer diagnosticsMu.Unlock()
		diagnostics = append(diagnostics, formatDiagnosticEvent(source, stage, time.Now()))
		if len(diagnostics) > 24 {
			diagnostics = diagnostics[len(diagnostics)-24:]
		}
	}

	// Enter the alternate screen buffer, as vim/htop/less do. It gives the
	// redraw loop a viewport with no scrollback of its own, so `\x1b[H` always
	// means the top-left cell the user is looking at, and it restores the
	// user's shell output untouched on exit.
	_, enterErr := fmt.Fprint(out, "\033[?1049h\033[?25l\033[2J\033[H")
	screenRestored := false
	restoreScreen := func() {
		if !screenRestored {
			fmt.Fprint(out, "\033[0m\033[?25h\033[?1049l")
			screenRestored = true
		}
	}
	defer restoreScreen()

	if enterErr != nil {
		return enterErr
	}

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

	draw := func() error {
		secLock.Lock()
		activeSec := sec
		showControls := overlayOpen
		showDiagnostics := diagnosticsOpen
		activeDebugOverlay := debugOverlay
		activeDiagnosticsOffset := diagnosticsOffset
		secLock.Unlock()
		diagnosticsMu.Lock()
		diagnosticSnapshot := append([]string(nil), diagnostics...)
		diagnosticsMu.Unlock()

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
		frameOpts := inheritUsageView(opts, WatchOptions{
			Host:                currentHost,
			Loading:             loading,
			ProcCounts:          lastProcs,
			ShowControls:        showControls,
			ShowDiagnostics:     showDiagnostics,
			Diagnostics:         diagnosticSnapshot,
			DiagnosticsOffset:   activeDiagnosticsOffset,
			RemoteLoadHost:      remoteLoadHost,
			RemoteLoadSnapshot:  remoteSnap,
			RemoteLoadStreaming: remoteStreaming,
			DebugOverlay:        activeDebugOverlay,
			MicLiveLevel:        micLive.Level,
			MicLiveAvailable:    micLive.Available,
		})
		frame := buildWatchFrame(lastSummary, lastRates, interval, activeSec, cols, rows, true, frameOpts)
		err := frame.paint(out)
		throttler.MarkDrawn(time.Now())
		return err
	}

	var splashStatusMu sync.Mutex
	var splashStatus splashStatusState
	reportFetchStage := func(source string, stage FetchStage) {
		appendDiagnostic(source, stage)
		splashStatusMu.Lock()
		splashStatus = splashStatusRecord(splashStatus, source, stage)
		splashStatusMu.Unlock()
	}
	splashEstimate, splashHaveEstimate := loadFetchDurationEstimate(homeDir, fetchDurationKindForHost(configuredHost))
	paintSplash := func(elapsed time.Duration, animate bool) (drained bool, err error) {
		cols, rows := terminalSize(out)
		now := time.Now()
		splashStatusMu.Lock()
		splashStatus = splashStatusAdvance(splashStatus, now, splashStatusMinDisplay)
		statusText := splashStatusLine(splashStatus.current.source, splashStatus.current.stage, splashStatus.have)
		badgesText := splashBadgesLine(splashStatus.badges)
		drained = splashStatusDrained(splashStatus, now, splashStatusMinDisplay)
		splashStatusMu.Unlock()
		frame := buildSplashFrame(cols, rows, elapsed, animate, splashEstimate, splashHaveEstimate, statusText, badgesText)
		err = frame.paint(out)
		return drained, err
	}

	// One collector runs at a time. Results cross a channel; only the UI
	// goroutine owns frame state and writes to the terminal. Refreshes must
	// never block keyboard handling or restoration (including the first one).
	type fetchResult struct {
		summary UsageSummary
		procs   *AgentProcessCount
		host    string
	}
	results := make(chan fetchResult, 1)
	var fetchDone chan struct{}
	startFetch := func() {
		if fetchDone != nil {
			return
		}
		secLock.Lock()
		targetHost, procRequested := activeHost, sec.Processes
		secLock.Unlock()
		fetchDone = make(chan struct{})
		done := fetchDone
		go func() {
			defer close(done)
			result := fetchResult{host: targetHost}
			if targetHost != "" {
				result.summary, result.procs, _ = CollectRemoteProgress(sigCtx, targetHost, procRequested, reportFetchStage)
			} else {
				if opts.StoredUsageLoader != nil {
					result.summary = opts.StoredUsageLoader(sigCtx)
				} else {
					result.summary = storedSummaryOrError(sigCtx)
				}
			}
			if sigCtx.Err() != nil {
				return
			}
			select {
			case results <- result:
			case <-sigCtx.Done():
			}
		}()
	}
	defer func() {
		stop()
		// Restore before draining canceled collectors, not after any final
		// filesystem/database work they may still need to unwind.
		restoreScreen()
		restoreTerminal()
		if fetchDone != nil {
			<-fetchDone
		}
	}()
	startFetch()

	if opts.Splash {
		splashStart := time.Now()
		splashTicker := time.NewTicker(splashFrameInterval)
		defer splashTicker.Stop()
		if _, err := paintSplash(0, true); err != nil {
			return err
		}
		fetchDoneLatched := false
	splashLoop:
		for {
			select {
			case <-sigCtx.Done():
				return nil
			case result := <-results:
				<-fetchDone
				fetchDone = nil
				currentHost, lastProcs = result.host, result.procs
				lastSummary = applyStaleQuota(result.summary, lastSummary)
				lastRates = tracker.update(lastSummary)
				loading = false
				fetchDoneLatched = true
				if drained, err := paintSplash(time.Since(splashStart), true); err != nil {
					return err
				} else if drained {
					splashActive.Store(false)
					if err := draw(); err != nil {
						return err
					}
					break splashLoop
				}
			case <-splashSkip:
				splashActive.Store(false)
				if _, err := paintSplash(time.Since(splashStart), false); err != nil {
					return err
				}
				if fetchDoneLatched {
					if err := draw(); err != nil {
						return err
					}
					break splashLoop
				}
				select {
				case result := <-results:
					<-fetchDone
					fetchDone = nil
					currentHost, lastProcs = result.host, result.procs
					lastSummary = applyStaleQuota(result.summary, lastSummary)
					lastRates = tracker.update(lastSummary)
					loading = false
					if err := draw(); err != nil {
						return err
					}
					break splashLoop
				case <-sigCtx.Done():
					return nil
				}
			case <-splashTicker.C:
				drained, err := paintSplash(time.Since(splashStart), true)
				if err != nil {
					return err
				}
				if fetchDoneLatched && drained {
					splashActive.Store(false)
					if err := draw(); err != nil {
						return err
					}
					break splashLoop
				}
			}
		}
	} else {
		if err := draw(); err != nil {
			return err
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
		case result := <-results:
			<-fetchDone
			fetchDone = nil
			currentHost, lastProcs = result.host, result.procs
			lastSummary = applyStaleQuota(result.summary, lastSummary)
			lastRates = tracker.update(lastSummary)
			loading = false
			if err := draw(); err != nil {
				return err
			}
			secLock.Lock()
			hostChanged := activeHost != currentHost
			secLock.Unlock()
			if hostChanged {
				startFetch()
			}
		case <-fetchChan:
			startFetch()
		case <-redrawChan:
			if err := draw(); err != nil {
				return err
			}
		case <-loadTicker.C:
			if err := draw(); err != nil {
				return err
			}
		case <-ticker.C:
			startFetch()
		}
	}
}
