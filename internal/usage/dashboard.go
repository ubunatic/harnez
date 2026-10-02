package usage

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// actionsPanelKey is the panel ID of the dashboard view's Actions box (issue 687).
const actionsPanelKey = "actions"

// dashboardAction is one button of the Actions box. The spec defines its
// label, its hotkey, and the harnez arguments it runs. Actions always run the
// current harnez executable, never a shell or another program.
type dashboardAction struct {
	Title string   `yaml:"title"`
	Key   string   `yaml:"key"`
	Args  []string `yaml:"args"`
}

func (a dashboardAction) label() string { return "[" + a.Key + "] " + a.Title }

func (a dashboardAction) commandLine() string { return "harnez " + strings.Join(a.Args, " ") }

// validateDashboardActions rejects actions whose hotkey is not a single
// printable ASCII character, is duplicated, or is already bound in
// spec/actions.yaml, and actions whose args do not start with a subcommand.
func validateDashboardActions(actions []dashboardAction) error {
	if len(actions) == 0 {
		return fmt.Errorf("actions must define at least one action")
	}
	seen := make(map[string]bool, len(actions))
	for i, a := range actions {
		if strings.TrimSpace(a.Title) == "" {
			return fmt.Errorf("actions[%d]: title must not be empty", i)
		}
		if len(a.Key) != 1 || a.Key[0] <= ' ' || a.Key[0] > '~' {
			return fmt.Errorf("actions[%d]: key must be one printable ASCII character", i)
		}
		if seen[a.Key] {
			return fmt.Errorf("actions[%d]: duplicate key %q", i, a.Key)
		}
		seen[a.Key] = true
		if bound := mustWatchActions().actionForKey(a.Key[0]); bound != "" {
			return fmt.Errorf("actions[%d]: key %q is already bound to %s in spec/actions.yaml", i, a.Key, bound)
		}
		if len(a.Args) == 0 || a.Args[0] == "" || strings.HasPrefix(a.Args[0], "-") {
			return fmt.Errorf("actions[%d]: args must start with a harnez subcommand", i)
		}
	}
	return nil
}

// dashboardActions returns the Actions box entries of the given view mode,
// or nil when the mode has no Actions panel.
func dashboardActions(mode usageViewModeSpec) []dashboardAction {
	if !mode.hasPanel(actionsPanelKey) {
		return nil
	}
	return mode.Actions
}

func dashboardActionForKey(actions []dashboardAction, key byte) (dashboardAction, bool) {
	for _, a := range actions {
		if a.Key[0] == key {
			return a, true
		}
	}
	return dashboardAction{}, false
}

func buildDashboardActionsBox(actions []dashboardAction, width int) wbox {
	lines := make([]string, 0, len(actions))
	for _, a := range actions {
		lines = append(lines, a.label())
	}
	return wbox{title: "Actions", lines: lines, width: width}
}

// dashboardHit is the clickable screen area of one action, in 0-based cells.
type dashboardHit struct {
	X, Y, W int
	Key     byte
}

// dashboardHits places one hit area per action line of an Actions box whose
// top border is at row top and whose left border is at column left.
func dashboardHits(actions []dashboardAction, top, left, width int) []dashboardHit {
	hits := make([]dashboardHit, 0, len(actions))
	for i, a := range actions {
		hits = append(hits, dashboardHit{X: left + 2, Y: top + 1 + i, W: max(1, width-4), Key: a.Key[0]})
	}
	return hits
}

func hitTest(hits []dashboardHit, x, y int) byte {
	for _, h := range hits {
		if y == h.Y && x >= h.X && x < h.X+h.W {
			return h.Key
		}
	}
	return 0
}

// mouseReport is one SGR (mode 1006) mouse report with 0-based coordinates.
type mouseReport struct {
	Button  int
	X, Y    int
	Release bool
}

// leftClick reports a plain left-button press or release, excluding motion,
// wheel, and other buttons.
func (m mouseReport) leftClick() bool { return m.Button&(3|32|64) == 0 }

// ttyEvent is either one key byte or one mouse report.
type ttyEvent struct {
	Key   byte
	Mouse *mouseReport
}

// ttyInput splits raw terminal input into key bytes and SGR mouse reports
// ("ESC [ < b ; x ; y M|m"). Every byte outside a mouse report passes
// through as a key, exactly as the byte-at-a-time reader handled it before.
// An incomplete report at the end of a chunk, or a chunk that is exactly
// "ESC [", waits for the next chunk.
type ttyInput struct {
	pending []byte
}

const maxMouseReportLen = 32

func (in *ttyInput) feed(chunk []byte) []ttyEvent {
	buf := append(in.pending, chunk...)
	in.pending = nil
	var events []ttyEvent
	for len(buf) > 0 {
		if string(buf) == "\x1b[" {
			// Possibly a mouse report split after "ESC [": wait for more.
			in.pending = append([]byte(nil), buf...)
			return events
		}
		if !hasMousePrefix(buf) {
			events = append(events, ttyEvent{Key: buf[0]})
			buf = buf[1:]
			continue
		}
		end := 3
		for end < len(buf) && (buf[end] == ';' || buf[end] >= '0' && buf[end] <= '9') {
			end++
		}
		if end == len(buf) && len(buf) < maxMouseReportLen {
			in.pending = append([]byte(nil), buf...)
			return events
		}
		if end == len(buf) || buf[end] != 'M' && buf[end] != 'm' {
			// Malformed report: drop its prefix and parameters, keep the rest.
			buf = buf[end:]
			continue
		}
		if m, ok := parseMouseReport(string(buf[3:end]), buf[end] == 'm'); ok {
			events = append(events, ttyEvent{Mouse: &m})
		}
		buf = buf[end+1:]
	}
	return events
}

// hasMousePrefix reports whether buf starts with "ESC [ <". A shorter tail
// such as a lone ESC stays a key, so the Esc key never waits for more input.
func hasMousePrefix(buf []byte) bool {
	const prefix = "\x1b[<"
	if len(buf) < len(prefix) {
		return false
	}
	return string(buf[:len(prefix)]) == prefix
}

func parseMouseReport(params string, release bool) (mouseReport, bool) {
	parts := strings.Split(params, ";")
	if len(parts) != 3 {
		return mouseReport{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return mouseReport{}, false
		}
		n[i] = v
	}
	return mouseReport{Button: n[0], X: n[1] - 1, Y: n[2] - 1, Release: release}, true
}

// runDashboardAction runs the action's harnez command with its output on out.
func runDashboardAction(ctx context.Context, out io.Writer, a dashboardAction) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve harnez executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, exe, a.Args...)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
