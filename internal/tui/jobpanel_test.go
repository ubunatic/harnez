package tui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/tui/jobs"
	"ubunatic.com/harnez/internal/tui/router"
	"ubunatic.com/loom"
)

// asyncLoop is a non-blocking Post stand-in; drain runs callbacks on the
// test goroutine, which plays the UI loop.
type asyncLoop struct {
	mu sync.Mutex
	q  []func()
}

func (l *asyncLoop) post(fn func()) {
	l.mu.Lock()
	l.q = append(l.q, fn)
	l.mu.Unlock()
}

func (l *asyncLoop) drain() int {
	l.mu.Lock()
	q := l.q
	l.q = nil
	l.mu.Unlock()
	for _, fn := range q {
		fn()
	}
	return len(q)
}

func (l *asyncLoop) until(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if l.drain() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
	}
}

func newJobsApp(t *testing.T) (*AppWidget, *asyncLoop) {
	t.Helper()
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	spec.Jobs.CancelGraceMS = 300
	app := NewAppWidget(spec, "claude:sonnet:low", "s", nil)
	l := &asyncLoop{}
	app.AttachRunner(func(context.Context, string, string, func(router.Chunk)) error {
		t.Error("a job prompt reached the model")
		return nil
	}, l.post, router.ResolveSpec)
	app.AttachJobs(NewJobManager(spec.Jobs, t.TempDir(), l.post))
	t.Cleanup(app.Shutdown)
	return app, l
}

func lastTurn(app *AppWidget) Turn {
	turns := app.Chat.Turns()
	return turns[len(turns)-1]
}

func screen(app *AppWidget) string {
	return strings.Join(RenderCanvas(app, 80, 20), "\n")
}

func press(app *AppWidget, keys ...string) {
	for _, k := range keys {
		if len([]rune(k)) == 1 {
			app.ConsumeKey(loom.KeyEvent{Text: k})
			continue
		}
		app.ConsumeKey(loom.KeyEvent{Key: k})
	}
}

func TestParseJobPrompt(t *testing.T) {
	cases := []struct {
		in, cmd string
		ok      bool
	}{
		{"!make test", "make test", true},
		{"  !  ls -l  ", "ls -l", true},
		{"!", "", true},
		{"  !", "", true},
		{"make !test", "", false},
		{"hello", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		cmd, ok := parseJobPrompt(c.in, "!")
		if cmd != c.cmd || ok != c.ok {
			t.Errorf("parseJobPrompt(%q) = %q, %v; want %q, %v", c.in, cmd, ok, c.cmd, c.ok)
		}
	}
}

func TestBangPromptRunsJobNotModel(t *testing.T) {
	app, l := newJobsApp(t)
	typeLine(app, "  !echo hi; exit 4")
	if app.Busy() {
		t.Fatal("job prompt started a model turn")
	}
	if got := lastTurn(app); got.Role != RoleNote || got.Content != "job 1 started: echo hi; exit 4" {
		t.Fatalf("start note %#v", got)
	}
	if !strings.Contains(screen(app), "[jobs: 1 running]") {
		t.Fatalf("header lacks running count:\n%s", screen(app))
	}
	l.until(t, func() bool { return app.runningJobs() == 0 })
	got := lastTurn(app)
	if got.Role != RoleNote || !strings.HasPrefix(got.Content, "job 1 failed (exit 4, ") {
		t.Fatalf("done note %#v", got)
	}
	if strings.Contains(screen(app), "running]") {
		t.Fatal("header keeps the running count after the job ended")
	}
	if app.Prompt.Text() != "" {
		t.Fatalf("prompt kept %q", app.Prompt.Text())
	}
}

func TestBangAloneShowsUsage(t *testing.T) {
	app, _ := newJobsApp(t)
	typeLine(app, "!")
	if got := lastTurn(app); got.Role != RoleError || got.Content != app.Spec.Jobs.Labels.Usage {
		t.Fatalf("got %#v", got)
	}
	if len(app.jobList()) != 0 {
		t.Fatal("bare ! started a job")
	}
}

func TestJobPanelKeys(t *testing.T) {
	app, l := newJobsApp(t)
	typeLine(app, "!echo first-out")
	typeLine(app, "!echo started-2; sleep 30")
	l.until(t, func() bool { return app.jobs.Get(1).Status == jobs.Succeeded })

	press(app, "ctrl-b")
	if !app.panel.open || app.panel.sel != 1 {
		t.Fatalf("panel open=%v sel=%d", app.panel.open, app.panel.sel)
	}
	s := screen(app)
	for _, want := range []string{"Jobs", "1  succeeded", "echo first-out", "2  running", app.Spec.Jobs.Labels.PanelHint} {
		if !strings.Contains(s, want) {
			t.Fatalf("panel lacks %q:\n%s", want, s)
		}
	}

	press(app, "up", "enter")
	if app.panel.logID != 1 {
		t.Fatalf("enter opened log %d", app.panel.logID)
	}
	if s := screen(app); !strings.Contains(s, "Job 1 (succeeded)") || !strings.Contains(s, "first-out") {
		t.Fatalf("log view:\n%s", s)
	}
	press(app, "esc")
	if !app.panel.open || app.panel.logID != 0 {
		t.Fatal("esc in the log should return to the list")
	}

	press(app, "down", "enter")
	l.until(t, func() bool { return strings.Contains(screen(app), "started-2") })
	press(app, "x")
	l.until(t, func() bool { return app.jobs.Get(2).Status != jobs.Running })
	if st := app.jobs.Get(2).Status; st != jobs.Cancelled {
		t.Fatalf("job 2 %v", st)
	}
	if got := lastTurn(app); !strings.HasPrefix(got.Content, "job 2 cancelled (exit signal") {
		t.Fatalf("cancel note %#v", got)
	}

	press(app, "esc", "esc")
	if app.panel.open || app.QuitRequested {
		t.Fatalf("open=%v quit=%v after esc esc", app.panel.open, app.QuitRequested)
	}
	press(app, "esc")
	if !app.QuitRequested {
		t.Fatal("esc with the panel closed should quit")
	}
}

func TestJobPanelSwallowsTyping(t *testing.T) {
	app, _ := newJobsApp(t)
	press(app, "ctrl-b", "a", "b", "enter")
	if app.Prompt.Text() != "" {
		t.Fatalf("panel leaked keys into the prompt: %q", app.Prompt.Text())
	}
	if !strings.Contains(screen(app), app.Spec.Jobs.Labels.PanelEmpty) {
		t.Fatalf("empty panel:\n%s", screen(app))
	}
	press(app, "ctrl-b")
	if app.panel.open {
		t.Fatal("toggle key should close the panel")
	}
}

func TestJobRowDisplayWidth(t *testing.T) {
	j := &jobs.Job{ID: 7, Command: "echo 日本語日本語日本語日本語", Status: jobs.Running, Start: time.Now()}
	for _, w := range []int{20, 31, 40, 80} {
		row := jobRow(j, w, j.Start.Add(2*time.Second))
		if got := loom.StringWidth(row); got != w {
			t.Errorf("width %d: row %q has width %d", w, row, got)
		}
	}
}

func TestLogViewFollowsTail(t *testing.T) {
	app, l := newJobsApp(t)
	typeLine(app, "!i=0; while test $i -lt 200; do echo row-$i; i=$((i+1)); done")
	l.until(t, func() bool { return app.runningJobs() == 0 })
	press(app, "ctrl-b", "enter")
	if s := screen(app); !strings.Contains(s, "row-199") {
		t.Fatalf("log should show the tail:\n%s", s)
	}
	press(app, "pgup")
	if app.panel.follow || strings.Contains(screen(app), "row-199") {
		t.Fatal("pgup should stop following")
	}
	press(app, "end")
	if !strings.Contains(screen(app), "row-199") {
		t.Fatal("end should follow the tail again")
	}
}

// Job output and state reach the app only through post: rendering on the
// test goroutine while jobs stream must be race-free under -race.
func TestJobUpdatesArePostOnly(t *testing.T) {
	app, l := newJobsApp(t)
	for i := 0; i < 3; i++ {
		typeLine(app, "!i=0; while test $i -lt 300; do echo out-$i; i=$((i+1)); done")
	}
	press(app, "ctrl-b", "enter")
	l.until(t, func() bool {
		_ = screen(app)
		return app.runningJobs() == 0
	})
	l.drain()
	if s := screen(app); !strings.Contains(s, "out-299") {
		t.Fatalf("final log:\n%s", s)
	}
}

func TestJobPanelBlocksPasteAndFlattensCommand(t *testing.T) {
	app, l := newJobsApp(t)
	app.Prompt.SetText("!echo a\n\techo   b")
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	l.until(t, func() bool { return app.runningJobs() == 0 })
	press(app, "ctrl-b")
	app.ConsumePaste(loom.PasteEvent{Text: "pasted"})
	if app.Prompt.Text() != "" {
		t.Fatalf("paste reached the hidden prompt: %q", app.Prompt.Text())
	}
	if !strings.Contains(screen(app), "echo a echo b") {
		t.Fatalf("command not flattened:\n%s", screen(app))
	}
	if got := lastTurn(app).Content; strings.ContainsAny(got, "\n\t") {
		t.Fatalf("done note %q", got)
	}
}

func TestJobsUnavailableUsesSpecLabel(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWidget(spec, "m", "s", nil)
	typeLine(app, "!true")
	if got := lastTurn(app); got.Role != RoleError || got.Content != spec.Jobs.Labels.Unavailable {
		t.Fatalf("got %#v", got)
	}
}
