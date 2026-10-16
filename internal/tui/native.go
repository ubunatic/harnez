package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/tui/router"
)

// NativeLookup returns the provider CLI command resuming a model's backend
// session (router.Router.Native).
type NativeLookup func(model string) (router.NativeCommand, error)

// NativeRun runs a native command with the terminal attached.
type NativeRun func(router.NativeCommand) error

// AttachNative enables the open_native key: suspend hands the terminal to the
// provider's own interactive CLI (loom Pane.Suspend); run nil runs it on the
// controlling terminal.
func (app *AppWidget) AttachNative(suspend func(func() error) error, lookup NativeLookup, run NativeRun) {
	if run == nil {
		run = runNative
	}
	app.suspend, app.nativeLookup, app.nativeRun = suspend, lookup, run
}

// openNative runs the active model's backend session in its provider CLI.
// Turns run non-interactively, so there is no live PTY to foreground: the
// native CLI resumes the stored session instead. The stored id is kept
// afterwards even if the CLI moved on to another id (e.g. a fork): the tui
// cannot learn that id reliably, and the stored session stays resumable.
// Header status and backend label are left untouched.
func (app *AppWidget) openNative() {
	labels := app.Spec.Native.Labels
	if app.Busy() {
		AddTurn(app.Chat, RoleNote, "", labels.Busy)
		return
	}
	if app.suspend == nil || app.nativeLookup == nil {
		AddTurn(app.Chat, RoleError, "", labels.Unavailable)
		return
	}
	nc, err := app.nativeLookup(app.Model)
	note := func(label string) string { return strings.ReplaceAll(label, "{{ .Provider }}", nc.Provider) }
	switch {
	case errors.Is(err, subagent.ErrInteractiveUnsupported):
		AddTurn(app.Chat, RoleNote, "", note(labels.Unsupported))
		return
	case errors.Is(err, router.ErrNoSession):
		if errors.Is(err, router.ErrInvalidSession) {
			AddTurn(app.Chat, RoleNote, "", note(labels.InvalidSession))
		}
		AddTurn(app.Chat, RoleNote, "", note(labels.NoSession))
		return
	case err != nil:
		AddTurn(app.Chat, RoleError, "", err.Error())
		return
	}
	run := app.nativeRun
	if err := app.suspend(func() error { return run(nc) }); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			err = fmt.Errorf("%s exited with status %d", nc.Argv[0], exit.ExitCode())
		}
		AddTurn(app.Chat, RoleError, "", err.Error())
	}
	AddTurn(app.Chat, RoleNote, "", note(labels.Returned))
}

// runNative runs nc on the controlling terminal with the provider's
// interactive environment (the agy launch env; others inherit). Unlike
// `harnez agent resume -i`, agy runs here without the metering supervisor and
// HARNEZ_SESSION_ID, so time spent in the native CLI is not metered.
func runNative(nc router.NativeCommand) error {
	env, err := subagent.InteractiveEnv(nc.Argv[0])
	if err != nil {
		return err
	}
	return runOnTTY(nc.Argv, nc.Dir, env)
}
