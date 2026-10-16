package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/tui/router"
	"ubunatic.com/loom"
)

// loop stands in for loom's event loop: post queues, drain runs callbacks.
type loop struct{ q chan func() }

func newLoop() *loop           { return &loop{q: make(chan func(), 64)} }
func (l *loop) post(fn func()) { l.q <- fn }
func (l *loop) drainUntil(t *testing.T, app *AppWidget) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for app.Busy() {
		select {
		case fn := <-l.q:
			fn()
		case <-timeout:
			t.Fatal("turn did not finish")
		}
	}
}

func newRunnerApp(t *testing.T, run TurnFunc) (*AppWidget, *loop) {
	t.Helper()
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWidget(spec, "claude:sonnet:low", "s", nil)
	l := newLoop()
	app.AttachRunner(run, l.post, router.ResolveSpec)
	return app, l
}

func typeLine(app *AppWidget, s string) {
	app.insertText(s)
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
}

func TestPostedChunksRenderInChat(t *testing.T) {
	app, l := newRunnerApp(t, func(_ context.Context, model, prompt string, emit func(router.Chunk)) error {
		emit(router.Chunk{Kind: router.ChunkText, Text: "hello from " + model})
		emit(router.Chunk{Kind: router.ChunkActivity, Text: "running go test"})
		emit(router.Chunk{Kind: router.ChunkText, Text: "done: " + prompt})
		return nil
	})
	typeLine(app, "ping")
	if !app.Busy() || app.Status != "busy" {
		t.Fatalf("Busy=%v Status=%q after submit", app.Busy(), app.Status)
	}
	l.drainUntil(t, app)
	if app.Status != "idle" {
		t.Fatalf("Status = %q, want idle", app.Status)
	}
	var roles []string
	for _, tr := range app.Chat.Turns() {
		roles = append(roles, string(tr.Role))
	}
	if got := strings.Join(roles, ","); got != "user,assistant,tool,assistant" {
		t.Fatalf("turn roles = %s", got)
	}
	frame := strings.Join(RenderCanvas(app, 80, 24), "\n")
	for _, want := range []string{"hello from claude:sonnet:low", "running go test", "done: ping", "● ready"} {
		if !strings.Contains(frame, want) {
			t.Errorf("frame missing %q:\n%s", want, frame)
		}
	}
}

func TestCancelKeysStopRunningTurnWithoutQuitting(t *testing.T) {
	for _, key := range []string{"esc", "ctrl-c"} {
		t.Run(key, func(t *testing.T) {
			started := make(chan struct{})
			app, l := newRunnerApp(t, func(ctx context.Context, _, _ string, _ func(router.Chunk)) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			})
			typeLine(app, "long task")
			<-started
			app.ConsumeKey(loom.KeyEvent{Key: key})
			if app.QuitRequested {
				t.Fatalf("%s quit during a running turn", key)
			}
			l.drainUntil(t, app)
			turns := app.Chat.Turns()
			last := turns[len(turns)-1]
			if last.Role != RoleNote || last.Content != app.Spec.Chat.CancelledLabel || app.Status != "idle" {
				t.Fatalf("last turn = %#v, status %q", last, app.Status)
			}
			for _, l := range RenderCanvas(app, 60, 20) {
				if strings.Contains(l, app.Spec.Chat.ErrorPrefix) {
					t.Fatalf("cancel rendered under the error prefix: %q", l)
				}
			}
			app.ConsumeKey(loom.KeyEvent{Key: key})
			if !app.QuitRequested {
				t.Fatalf("%s when idle should quit", key)
			}
		})
	}
}

func TestBusySubmitKeepsPrompt(t *testing.T) {
	started := make(chan struct{})
	app, l := newRunnerApp(t, func(ctx context.Context, _, _ string, _ func(router.Chunk)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	typeLine(app, "first")
	<-started
	typeLine(app, "second")
	if got := app.Prompt.Text(); got != "second" {
		t.Fatalf("prompt = %q, want it kept", got)
	}
	for _, tr := range app.Chat.Turns() {
		if tr.Role == RoleUser && tr.Content == "second" {
			t.Fatal("busy submit added a user turn")
		}
	}
	app.CancelTurn()
	l.drainUntil(t, app)
}

func TestShutdownStopsRunningTurn(t *testing.T) {
	started := make(chan struct{})
	stopped := false
	app, _ := newRunnerApp(t, func(ctx context.Context, _, _ string, _ func(router.Chunk)) error {
		close(started)
		<-ctx.Done()
		stopped = true
		return ctx.Err()
	})
	typeLine(app, "work")
	<-started
	done := make(chan struct{})
	go func() { app.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return")
	}
	if !stopped {
		t.Fatal("Shutdown returned before the turn stopped")
	}
}

func TestTurnErrorRendersAsErrorTurn(t *testing.T) {
	app, l := newRunnerApp(t, func(context.Context, string, string, func(router.Chunk)) error {
		return errors.New("claude: exit status 1")
	})
	typeLine(app, "x")
	l.drainUntil(t, app)
	frame := strings.Join(RenderCanvas(app, 80, 24), "\n")
	if !strings.Contains(frame, "Error:") || !strings.Contains(frame, "claude: exit status 1") || app.Status != "error" {
		t.Fatalf("status %q, frame:\n%s", app.Status, frame)
	}
}

func TestModelCommand(t *testing.T) {
	var gotModel string
	app, l := newRunnerApp(t, func(_ context.Context, model, _ string, _ func(router.Chunk)) error {
		gotModel = model
		return nil
	})
	typeLine(app, "/model codex:luna:high")
	if app.Model != "codex:luna:high" || app.Busy() {
		t.Fatalf("Model = %q busy=%v", app.Model, app.Busy())
	}
	typeLine(app, "/model nosuch:thing")
	typeLine(app, "/model")
	if app.Model != "codex:luna:high" {
		t.Fatalf("invalid /model changed model to %q", app.Model)
	}
	turns := app.Chat.Turns()
	if len(turns) != 3 || turns[1].Role != RoleError || !strings.HasPrefix(turns[2].Content, "usage: /model") {
		t.Fatalf("turns = %#v", turns)
	}
	typeLine(app, "hi")
	l.drainUntil(t, app)
	if gotModel != "codex:luna:high" {
		t.Fatalf("turn ran on %q", gotModel)
	}
	if !strings.Contains(app.formatHeader(), "codex:luna:high") {
		t.Fatalf("header = %q", app.formatHeader())
	}
}

// The header says whether the last turn resumed its backend session or
// started a new one; session markers never reach the chat.
func TestSessionChunksShowInHeader(t *testing.T) {
	fresh := false
	app, l := newRunnerApp(t, func(_ context.Context, _, _ string, emit func(router.Chunk)) error {
		emit(router.Chunk{Kind: router.ChunkSession, Text: router.SessionResumed})
		if fresh {
			emit(router.Chunk{Kind: router.ChunkActivity, Text: "could not resume session x; starting a new session"})
			emit(router.Chunk{Kind: router.ChunkSession, Text: router.SessionNew})
		}
		emit(router.Chunk{Kind: router.ChunkText, Text: "ok"})
		return nil
	})
	if h := app.formatHeader(); strings.Contains(h, "[resumed]") || strings.Contains(h, "[new session]") || strings.HasSuffix(h, " ") {
		t.Fatalf("header before first turn = %q", h)
	}
	typeLine(app, "a")
	l.drainUntil(t, app)
	if h := app.formatHeader(); !strings.HasSuffix(h, "[resumed]") {
		t.Fatalf("header after resumed turn = %q", h)
	}
	fresh = true
	typeLine(app, "b")
	l.drainUntil(t, app)
	if h := app.formatHeader(); !strings.HasSuffix(h, "[new session]") {
		t.Fatalf("header after fresh turn = %q", h)
	}
	frame := strings.Join(RenderCanvas(app, 120, 24), "\n")
	if !strings.Contains(frame, "[new session]") {
		t.Fatalf("frame lacks backend label:\n%s", frame)
	}
	for _, tr := range app.Chat.Turns() {
		if tr.Content == router.SessionResumed || tr.Content == router.SessionNew {
			t.Fatalf("session marker rendered as chat turn %#v", tr)
		}
	}
}

// The built-in command list lives in the spec only: a spec builtin is a
// command, an unlisted /name is plain prompt text.
func TestIsCommandDerivedFromSpec(t *testing.T) {
	app, _ := newRunnerApp(t, nil)
	for text, want := range map[string]bool{"/model x": true, "/model": true, "  /model  a b": true, "/models": false, "/other": false, "model": false, "": false} {
		if got := app.isCommand(text); got != want {
			t.Errorf("isCommand(%q) = %v, want %v", text, got, want)
		}
	}
	app.Spec.Picker.Builtins = append(app.Spec.Picker.Builtins, PickerCommand{Name: "other", Usage: "/other"})
	if !app.isCommand("/other") {
		t.Error("a builtin added to the spec must count as a command")
	}
	app.Spec.Picker.Builtins = nil
	if app.isCommand("/model x") {
		t.Error("with no spec builtins nothing is a command")
	}
}
