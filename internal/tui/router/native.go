package router

import (
	"errors"
	"fmt"

	"ubunatic.com/harnez/internal/subagent"
)

// ErrNoSession means the model has no backend session to open natively yet.
var ErrNoSession = errors.New("no backend session yet")

// NativeCommand is the provider CLI's own interactive session resuming the
// backend session the next tui turn would resume.
type NativeCommand struct {
	Provider  string
	SessionID string
	Dir       string
	// Argv comes from subagent.InteractiveArgv, the builder of
	// `harnez agent resume -i`; Argv[0] is the provider CLI.
	Argv []string
}

// Native returns the interactive resume command of spec's backend session.
// A provider without an interactive CLI session returns an error wrapping
// subagent.ErrInteractiveUnsupported; a model without a session ErrNoSession.
// The returned Provider is set in both cases.
func (r *Router) Native(spec string) (NativeCommand, error) {
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return NativeCommand{}, err
	}
	nc := NativeCommand{Provider: m.Provider, SessionID: r.Session(spec), Dir: r.Dir}
	// With an empty id the builder returns the new-session argv, which is
	// discarded: the call only checks that the provider is supported.
	argv, err := subagent.InteractiveArgv(m, r.Dir, nc.SessionID)
	if err != nil {
		return nc, err
	}
	if nc.SessionID == "" {
		return nc, fmt.Errorf("%s: %w", m.Provider, ErrNoSession)
	}
	nc.Argv = argv
	return nc, nil
}
