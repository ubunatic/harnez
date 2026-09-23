package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"codeberg.org/ubunatic/loom"
	"golang.org/x/term"
)

// UsageLoomWidget implements loom.Widget for the compact usage monitor loom app.
type UsageLoomWidget struct {
	Summary UsageSummary
	Rates   map[string]agentRate
	Opts    WatchOptions
	HomeDir string
	Now     time.Time
}

// NewUsageLoomWidget creates a loom.Widget for displaying the compact usage monitor.
func NewUsageLoomWidget(summary UsageSummary, opts WatchOptions, homeDir string) *UsageLoomWidget {
	opts.Compact = true
	return &UsageLoomWidget{
		Summary: summary,
		Opts:    opts,
		HomeDir: homeDir,
		Now:     time.Now(),
	}
}

// Draw renders the compact usage frame onto the loom canvas.
func (w *UsageLoomWidget) Draw(c *loom.Canvas, r loom.Rect) {
	sec := compactWatchSections()
	now := w.Now
	if now.IsZero() {
		now = time.Now()
	}
	frame := buildWatchFrameAt(w.Summary, w.Rates, 0, sec, r.W, r.H, false, w.HomeDir, "", now, w.Opts)
	for i, line := range frame.lines {
		if i >= r.H {
			break
		}
		c.WriteANSI(r.X, r.Y+i, line)
	}
}

// HandleKey processes keyboard shortcuts (q, Esc, Ctrl-C to exit).
func (w *UsageLoomWidget) HandleKey(e loom.KeyEvent) bool {
	if e.Is("q", "esc", "ctrl-c") || e.Text == "q" || e.Text == "Q" {
		return true
	}
	return false
}

// HandleMouse handles mouse events.
func (w *UsageLoomWidget) HandleMouse(_ loom.MouseEvent) bool {
	return false
}

// ContentHeight reports the preferred height of the compact usage monitor frame.
func (w *UsageLoomWidget) ContentHeight() int {
	sec := compactWatchSections()
	now := w.Now
	if now.IsZero() {
		now = time.Now()
	}
	frame := buildWatchFrameAt(w.Summary, w.Rates, 0, sec, 90, 24, false, w.HomeDir, "", now, w.Opts)
	return len(frame.lines)
}

// RenderLoom renders the usage monitor as a loom app widget into ANSI row strings.
func RenderLoom(summary UsageSummary, opts WatchOptions, cols, rows int) []string {
	widget := NewUsageLoomWidget(summary, opts, "")
	return loom.Render(widget, cols, rows)
}

// RunLoom starts the usage monitor as a loom app.
func RunLoom(ctx context.Context, homeDir string, client *http.Client, out io.Writer, interval time.Duration, opts WatchOptions) error {
	opts.Compact = true
	summary := CollectAll(ctx, homeDir, client)
	widget := NewUsageLoomWidget(summary, opts, homeDir)

	f, isFile := out.(*os.File)
	if !isFile || !term.IsTerminal(int(f.Fd())) {
		cols, rows := 90, 24
		if isFile {
			c, r := terminalSize(out)
			if c > 0 {
				cols = c
			}
			if r > 0 {
				rows = r
			}
		}
		lines := loom.Render(widget, cols, rows)
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		return nil
	}

	height := widget.ContentHeight()
	pane, err := loom.New(height)
	if err != nil {
		lines := loom.Render(widget, 90, 24)
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		return nil
	}
	defer pane.Close()

	if opts.Host != "" {
		cadence := loom.Cadence{
			Collect: interval,
			Redraw:  time.Second,
		}
		return pane.RunWatch(ctx, widget, cadence, func(now time.Time) error {
			s, _, err := CollectRemote(ctx, opts.Host, opts.ShowProcesses)
			if err == nil {
				widget.Summary = s
				widget.Now = now
			}
			return nil
		})
	}

	return pane.Run(widget)
}
