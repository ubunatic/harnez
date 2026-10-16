package tui

import (
	"context"
	"errors"
	"strings"
	"sync"

	"ubunatic.com/harnez/internal/tui/router"
)

// TurnFunc runs prompt on model and emits output chunks in order.
type TurnFunc func(ctx context.Context, model, prompt string, emit func(router.Chunk)) error

// turnState tracks the one running turn of an AppWidget.
type turnState struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// AttachRunner makes submitted prompts run through run in a goroutine.
// post must run its callback on the UI loop (loom Pane.Post); every widget
// mutation of a running turn goes through it. resolve validates /model specs.
func (app *AppWidget) AttachRunner(run TurnFunc, post func(func()), resolve func(string) (string, error)) {
	app.run, app.post, app.resolve = run, post, resolve
}

// Busy reports whether a turn is running.
func (app *AppWidget) Busy() bool { return app.turn != nil }

// CancelTurn stops the running turn, if any; its error turn follows via post.
func (app *AppWidget) CancelTurn() {
	if app.turn != nil {
		app.turn.cancel()
	}
}

// Shutdown cancels a running turn and waits until its provider process and
// goroutine are gone. Call it after the pane loop has stopped.
func (app *AppWidget) Shutdown() {
	if t := app.turn; t != nil {
		t.cancel()
		t.wg.Wait()
	}
}

// handleCommand runs a slash command and reports whether text was one.
func (app *AppWidget) handleCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || fields[0] != "/model" {
		return false
	}
	if len(fields) != 2 {
		AddTurn(app.Chat, RoleError, "", "usage: /model <provider:model[:tier]> (current: "+app.Model+")")
		return true
	}
	spec := fields[1]
	if app.resolve != nil {
		resolved, err := app.resolve(spec)
		if err != nil {
			AddTurn(app.Chat, RoleError, "", err.Error())
			return true
		}
		spec = resolved
	}
	app.Model = spec
	AddTurn(app.Chat, RoleTool, "model", "active model: "+spec)
	return true
}

// startTurn runs prompt on the active model; output arrives through post.
func (app *AppWidget) startTurn(prompt string) {
	if app.turn != nil {
		AddTurn(app.Chat, RoleError, "", "a turn is still running; press Esc to cancel it")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &turnState{cancel: cancel}
	app.turn = t
	app.Status = "busy"
	model := app.Model
	run, post := app.run, app.post
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer cancel()
		err := run(ctx, model, prompt, func(c router.Chunk) {
			post(func() { app.applyChunk(model, c) })
		})
		post(func() { app.finishTurn(t, err) })
	}()
}

func (app *AppWidget) applyChunk(model string, c router.Chunk) {
	switch c.Kind {
	case router.ChunkActivity:
		AddTurn(app.Chat, RoleTool, "exec", c.Text)
	default:
		AppendLast(app.Chat, RoleAssistant, model, c.Text)
	}
}

func (app *AppWidget) finishTurn(t *turnState, err error) {
	if app.turn == t {
		app.turn = nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		AddTurn(app.Chat, RoleError, "", "turn cancelled")
		app.Status = "idle"
	case err != nil:
		AddTurn(app.Chat, RoleError, "", err.Error())
		app.Status = "error"
	default:
		app.Status = "idle"
	}
}
