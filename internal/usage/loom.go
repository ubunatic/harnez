package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"codeberg.org/ubunatic/loom"
	"golang.org/x/term"
)

// UsageLoomWidget implements loom.Widget for the compact usage monitor loom app.
type UsageLoomWidget struct {
	mu sync.Mutex

	st watchKeyState

	summary    UsageSummary
	rates      map[string]agentRate
	procCounts *AgentProcessCount
	interval   time.Duration

	Opts       WatchOptions
	HomeDir    string
	HistoryDir string
	Now        time.Time
	Live       bool

	// Splash screen state
	splashActive       bool
	splashElapsed      time.Duration
	splashAnimate      bool
	splashStatusText   string
	splashBadgesText   string
	splashEstimate     time.Duration
	splashHaveEstimate bool

	// Callbacks for side-effects triggered by keypresses
	OnFetch func()
}

// NewUsageLoomWidget creates a loom.Widget for displaying the compact usage monitor.
func NewUsageLoomWidget(summary UsageSummary, opts WatchOptions, homeDir string) *UsageLoomWidget {
	opts.Compact = true
	sec := compactWatchSections()
	if opts.ShowProcesses {
		sec.Processes = true
	}
	if opts.ShowMic {
		sec.Mic = true
	}
	st := watchKeyState{
		sec:            sec,
		presetIdx:      1, // compact preset
		configuredHost: opts.Host,
		activeHost:     opts.Host,
	}
	return &UsageLoomWidget{
		summary:       summary,
		st:            st,
		Opts:          opts,
		HomeDir:       homeDir,
		Now:           time.Now(),
		splashAnimate: true,
	}
}

// SetSummary updates the current UsageSummary and recalculates rates.
func (w *UsageLoomWidget) SetSummary(s UsageSummary, rates map[string]agentRate) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.summary = s
	w.rates = rates
}

// Draw renders either the startup splash or the compact usage frame onto the loom canvas.
func (w *UsageLoomWidget) Draw(c *loom.Canvas, r loom.Rect) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.Now
	if now.IsZero() {
		now = time.Now()
	}

	if w.splashActive {
		frame := buildSplashFrame(r.W, r.H, w.splashElapsed, w.splashAnimate, w.splashEstimate, w.splashHaveEstimate, w.splashStatusText, w.splashBadgesText)
		for i, line := range frame.lines {
			if i >= r.H {
				break
			}
			c.WriteANSI(r.X, r.Y+i, line)
		}
		return
	}

	opts := w.Opts
	opts.ShowControls = w.st.overlayOpen
	opts.ShowDiagnostics = w.st.diagnosticsOpen
	opts.DiagnosticsOffset = w.st.diagnosticsOffset
	opts.DebugOverlay = w.st.debugOverlay
	opts.Host = w.st.activeHost
	opts.ProcCounts = w.procCounts

	frame := buildWatchFrameAt(w.summary, w.rates, w.interval, w.st.sec, r.W, r.H, w.Live, w.HomeDir, w.HistoryDir, now, opts)
	for i, line := range frame.lines {
		if i >= r.H {
			break
		}
		c.WriteANSI(r.X, r.Y+i, line)
	}
}

// keyToByte converts a loom.KeyEvent into a single byte hotkey if applicable.
func keyToByte(e loom.KeyEvent) byte {
	if e.Key != "" {
		switch e.Key {
		case "esc":
			return 27
		case "ctrl-c":
			return 3
		case "enter":
			return '\r'
		}
	}
	if len(e.Text) == 1 {
		return e.Text[0]
	}
	return 0
}

// HandleKey processes keyboard shortcuts in the TUI loop.
func (w *UsageLoomWidget) HandleKey(e loom.KeyEvent) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	b := keyToByte(e)
	if b == 0 {
		return false
	}

	if w.splashActive {
		eff := dispatchSplashKey(b)
		if eff.quit {
			return true
		}
		if eff.skip {
			w.splashAnimate = false
			w.splashActive = false
		}
		return false
	}

	newSt, eff := dispatchWatchKey(w.st, b, w.Opts.ShowProcesses)
	w.st = newSt

	if eff.quit {
		return true
	}
	if eff.fetch && w.OnFetch != nil {
		go w.OnFetch()
	}
	return false
}

// HandleMouse handles mouse events.
func (w *UsageLoomWidget) HandleMouse(_ loom.MouseEvent) bool {
	return false
}

// ContentHeight reports the preferred height of the compact usage monitor frame.
func (w *UsageLoomWidget) ContentHeight() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.Now
	if now.IsZero() {
		now = time.Now()
	}
	opts := w.Opts
	opts.ShowControls = w.st.overlayOpen
	opts.ShowDiagnostics = w.st.diagnosticsOpen
	opts.DiagnosticsOffset = w.st.diagnosticsOffset
	opts.DebugOverlay = w.st.debugOverlay
	opts.Host = w.st.activeHost

	frame := buildWatchFrameAt(w.summary, w.rates, w.interval, w.st.sec, 100, 24, w.Live, w.HomeDir, w.HistoryDir, now, opts)
	return len(frame.lines)
}

// RenderLoom renders the usage monitor as a loom app widget into ANSI row strings.
func RenderLoom(summary UsageSummary, opts WatchOptions, cols, rows int) []string {
	widget := NewUsageLoomWidget(summary, opts, "")
	if cols < 100 {
		cols = 100
	}
	if rows <= 0 {
		rows = widget.ContentHeight()
	}
	return loom.Render(widget, cols, rows)
}

// RenderLoomPrint performs a one-shot collection and prints the compact loom widget output to out.
func RenderLoomPrint(ctx context.Context, homeDir string, client *http.Client, out io.Writer, opts WatchOptions) error {
	opts.Compact = true
	summary := CollectAll(ctx, homeDir, client)
	cols, rows := terminalSize(out)
	if cols < 100 {
		cols = 100
	}
	if rows < 12 {
		rows = 24
	}
	widget := NewUsageLoomWidget(summary, opts, homeDir)
	lines := loom.Render(widget, cols, widget.ContentHeight())
	for _, l := range lines {
		fmt.Fprintln(out, l)
	}
	return nil
}

// RunLoom starts the usage monitor as a loom app (legacy fallback or watch).
func RunLoom(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, opts WatchOptions) error {
	return RenderLoomPrint(ctx, homeDir, client, out, opts)
}

// RunLoomWatch runs the interactive loom TUI usage monitor in watch mode.
func RunLoomWatch(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, historyDir string, opts WatchOptions) error {
	if interval < MinWatchInterval {
		interval = MinWatchInterval
	}
	opts.Compact = true

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	f, isFile := out.(*os.File)
	if !isFile || !term.IsTerminal(int(f.Fd())) {
		return RenderLoomPrint(sigCtx, homeDir, client, out, opts)
	}

	widget := NewUsageLoomWidget(UsageSummary{}, opts, homeDir)
	widget.HistoryDir = historyDir
	widget.interval = interval
	widget.Live = true

	height := 14
	pane, err := loom.New(height)
	if err != nil {
		return RenderLoomPrint(sigCtx, homeDir, client, out, opts)
	}
	defer pane.Close()

	tracker := newRateTracker()

	// Initial splash phase setup
	widget.splashActive = true
	widget.splashEstimate, widget.splashHaveEstimate = loadFetchDurationEstimate(homeDir, fetchDurationKindForHost(opts.Host))

	var splashStatus splashStatusState
	var splashStatusMu sync.Mutex
	reportStage := func(source string, stage FetchStage) {
		splashStatusMu.Lock()
		splashStatus = splashStatusRecord(splashStatus, source, stage)
		splashStatusMu.Unlock()
	}

	fetchStart := time.Now()
	firstFetchDone := make(chan struct{})

	go func() {
		defer close(firstFetchDone)
		var fresh UsageSummary
		if opts.Host != "" {
			var procRes *AgentProcessCount
			fresh, procRes, _ = CollectRemoteProgress(sigCtx, opts.Host, opts.ShowProcesses, reportStage)
			widget.mu.Lock()
			widget.procCounts = procRes
			widget.mu.Unlock()
		} else {
			fresh = CollectAllProgress(sigCtx, homeDir, client, reportStage)
			if historyDir != "" {
				_ = AppendHistory(historyDir, fresh)
			}
		}
		if sigCtx.Err() == nil {
			recordFetchDuration(homeDir, fetchDurationKindForHost(opts.Host), time.Since(fetchStart))
		}
		rates := tracker.update(fresh)
		widget.SetSummary(fresh, rates)
	}()

	// Animate splash until first fetch finishes
	splashTicker := time.NewTicker(splashFrameInterval)
	splashStart := time.Now()

splashLoop:
	for {
		select {
		case <-sigCtx.Done():
			splashTicker.Stop()
			return nil
		case <-firstFetchDone:
			splashTicker.Stop()
			widget.mu.Lock()
			widget.splashActive = false
			widget.mu.Unlock()
			break splashLoop
		case <-splashTicker.C:
			elapsed := time.Since(splashStart)
			splashStatusMu.Lock()
			now := time.Now()
			splashStatus = splashStatusAdvance(splashStatus, now, splashStatusMinDisplay)
			statusText := splashStatusLine(splashStatus.current.source, splashStatus.current.stage, splashStatus.have)
			badgesText := splashBadgesLine(splashStatus.badges)
			drained := splashStatusDrained(splashStatus, now, splashStatusMinDisplay)
			splashStatusMu.Unlock()

			widget.mu.Lock()
			widget.splashElapsed = elapsed
			widget.splashStatusText = statusText
			widget.splashBadgesText = badgesText
			if !widget.splashActive || drained {
				widget.splashActive = false
				widget.mu.Unlock()
				splashTicker.Stop()
				break splashLoop
			}
			widget.mu.Unlock()
		}
	}

	cadence := loom.Cadence{
		Collect: interval,
		Redraw:  time.Second,
	}

	widget.OnFetch = func() {
		var fresh UsageSummary
		widget.mu.Lock()
		host := widget.st.activeHost
		procRequested := widget.st.sec.Processes
		widget.mu.Unlock()

		if host != "" {
			var procRes *AgentProcessCount
			fresh, procRes, _ = CollectRemote(sigCtx, host, procRequested)
			widget.mu.Lock()
			widget.procCounts = procRes
			widget.mu.Unlock()
		} else {
			fresh = CollectAll(sigCtx, homeDir, client)
			if historyDir != "" {
				_ = AppendHistory(historyDir, fresh)
			}
		}
		rates := tracker.update(fresh)
		widget.SetSummary(fresh, rates)
	}

	return pane.RunWatch(sigCtx, widget, cadence, func(now time.Time) error {
		widget.mu.Lock()
		widget.Now = now
		widget.mu.Unlock()
		return nil
	})
}
