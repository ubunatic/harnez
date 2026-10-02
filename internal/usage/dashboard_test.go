package usage

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDashboardSpecActions(t *testing.T) {
	actions := dashboardActions(usageViewMode("dashboard", false, false))
	if len(actions) == 0 {
		t.Fatal("dashboard mode defines no actions")
	}
	if err := validateDashboardActions(actions); err != nil {
		t.Fatalf("embedded dashboard actions invalid: %v", err)
	}
	for _, mode := range []string{"normal", "compact", "minimal"} {
		if got := dashboardActions(usageViewMode(mode, false, false)); len(got) != 0 {
			t.Fatalf("mode %s has actions %+v", mode, got)
		}
	}
}

func TestValidateDashboardActionsRejects(t *testing.T) {
	ok := dashboardAction{Title: "History", Key: "h", Args: []string{"usage", "history"}}
	for _, tc := range []struct {
		name    string
		actions []dashboardAction
		want    string
	}{
		{"none", nil, "at least one"},
		{"empty title", []dashboardAction{{Title: " ", Key: "h", Args: ok.Args}}, "title"},
		{"long key", []dashboardAction{{Title: "x", Key: "hh", Args: ok.Args}}, "printable"},
		{"space key", []dashboardAction{{Title: "x", Key: " ", Args: ok.Args}}, "printable"},
		{"duplicate key", []dashboardAction{ok, ok}, "duplicate"},
		{"bound watch key", []dashboardAction{{Title: "x", Key: "a", Args: ok.Args}}, "toggle_all_usage"},
		{"no args", []dashboardAction{{Title: "x", Key: "h"}}, "subcommand"},
		{"flag first", []dashboardAction{{Title: "x", Key: "h", Args: []string{"--help"}}}, "subcommand"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDashboardActions(tc.actions)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
	if err := validateDashboardActions([]dashboardAction{ok}); err != nil {
		t.Fatalf("valid action rejected: %v", err)
	}
}

func TestTTYInputSplitsKeysAndMouse(t *testing.T) {
	var in ttyInput
	got := in.feed([]byte("q\x1b[<0;5;"))
	if len(got) != 1 || got[0].Key != 'q' {
		t.Fatalf("first chunk events = %+v", got)
	}
	got = in.feed([]byte("7M\x1b[<0;5;7mx"))
	want := []ttyEvent{
		{Mouse: &mouseReport{Button: 0, X: 4, Y: 6}},
		{Mouse: &mouseReport{Button: 0, X: 4, Y: 6, Release: true}},
		{Key: 'x'},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
	// A lone Esc and non-mouse escape sequences stay key bytes, as before.
	got = in.feed([]byte("\x1b[A\x1b"))
	var keys []byte
	for _, ev := range got {
		if ev.Mouse != nil {
			t.Fatalf("unexpected mouse event %+v", ev.Mouse)
		}
		keys = append(keys, ev.Key)
	}
	if string(keys) != "\x1b[A\x1b" {
		t.Fatalf("keys = %q", keys)
	}
}

func TestMouseReportLeftClick(t *testing.T) {
	for _, tc := range []struct {
		button int
		want   bool
	}{{0, true}, {1, false}, {2, false}, {32, false}, {64, false}, {65, false}} {
		if got := (mouseReport{Button: tc.button}).leftClick(); got != tc.want {
			t.Errorf("button %d leftClick = %t, want %t", tc.button, got, tc.want)
		}
	}
}

// TestDashboardHitsCoverActionLabels checks that every click area returned
// with a dashboard frame sits on that action's label in the painted lines.
func TestDashboardHitsCoverActionLabels(t *testing.T) {
	tempDir := t.TempDir()
	summary := UsageSummary{Timestamp: time.Now()}
	opt := WatchOptions{Mode: "dashboard", HistoryStats: &HistorySummaryData{}}
	frame := buildWatchFrame(summary, nil, time.Minute, initialWatchSections(opt), 120, 50, true, tempDir, tempDir, opt)
	actions := dashboardActions(usageViewMode("dashboard", false, false))
	if len(frame.hits) != len(actions) {
		t.Fatalf("hits = %+v, want one per action (%d)", frame.hits, len(actions))
	}
	for i, h := range frame.hits {
		line := []rune(stripANSI(frame.lines[h.Y]))
		text := string(line[h.X:min(len(line), h.X+h.W)])
		if !strings.HasPrefix(text, actions[i].label()) {
			t.Errorf("hit %d covers %q, want label %q", i, text, actions[i].label())
		}
		if got := hitTest(frame.hits, h.X, h.Y); got != actions[i].Key[0] {
			t.Errorf("hitTest(%d,%d) = %q, want %q", h.X, h.Y, got, actions[i].Key)
		}
	}
	plain := WatchOptions{Mode: "normal", HistoryStats: &HistorySummaryData{}}
	if f := buildWatchFrame(summary, nil, time.Minute, initialWatchSections(plain), 120, 50, true, tempDir, tempDir, plain); len(f.hits) != 0 {
		t.Fatalf("normal mode has hits %+v", f.hits)
	}
}

func TestTTYInputEdgeCases(t *testing.T) {
	var in ttyInput
	// A report split right after "ESC [" must not leak digits as keys.
	if got := in.feed([]byte("\x1b[")); len(got) != 0 {
		t.Fatalf("split prefix events = %+v", got)
	}
	got := in.feed([]byte("<0;2;3M"))
	if len(got) != 1 || got[0].Mouse == nil || got[0].Mouse.X != 1 || got[0].Mouse.Y != 2 {
		t.Fatalf("rejoined report events = %+v", got)
	}
	// A malformed report must not swallow later keys up to an "m".
	got = in.feed([]byte("\x1b[<0;2xqm"))
	var keys []byte
	for _, ev := range got {
		if ev.Mouse != nil {
			t.Fatalf("malformed report parsed as %+v", ev.Mouse)
		}
		keys = append(keys, ev.Key)
	}
	if string(keys) != "xqm" {
		t.Fatalf("keys after malformed report = %q", keys)
	}
}

func TestWatchScreenSeqsDisableMouseOnLeave(t *testing.T) {
	enter, leave := watchScreenSeqs(true)
	for _, mode := range []string{"1000", "1006"} {
		if !strings.Contains(enter, "\x1b[?"+mode+"h") || !strings.Contains(leave, "\x1b[?"+mode+"l") {
			t.Fatalf("mouse mode %s not paired: enter=%q leave=%q", mode, enter, leave)
		}
	}
	enter, leave = watchScreenSeqs(false)
	if strings.Contains(enter+leave, "\x1b[?100") {
		t.Fatalf("mouse sequences without mouse: enter=%q leave=%q", enter, leave)
	}
}
