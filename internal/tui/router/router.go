// Package router runs harnez tui turns through the existing subagent drivers.
//
// Subscription-bound providers (claude, codex, agy, pi) go through their CLI
// driver so a turn draws on the user's plan quota. Drivers that implement
// subagent.StreamingDriver (codex, claude, pi via subagent.StreamDriverFor)
// report messages and activity while the turn runs; batch drivers (agy)
// report the turn's messages when it completes.
// Providers without a driver get a clear "not supported yet" error.
package router

import (
	"context"
	"fmt"
	"sync"

	"ubunatic.com/harnez/internal/subagent"
)

// ChunkKind classifies a piece of turn output.
type ChunkKind string

const (
	// ChunkText is assistant message text; each chunk is one whole message.
	ChunkText ChunkKind = "text"
	// ChunkActivity describes a tool call or command the agent started.
	ChunkActivity ChunkKind = "activity"
	// ChunkSession says how the turn's backend session started: Text is
	// SessionResumed or SessionNew. A failed resume follows up with SessionNew.
	ChunkSession ChunkKind = "session"
)

// ChunkSession texts.
const (
	SessionResumed = "resumed"
	SessionNew     = "new"
)

// Chunk is one piece of turn output, delivered in arrival order.
type Chunk struct {
	Kind ChunkKind
	Text string
}

// Router routes prompts to the driver of the active model and keeps one
// backend session per provider model, so the next prompt resumes it natively
// (the provider CLI's resume, no history replay).
type Router struct {
	// Dir is the working directory the provider CLIs run in.
	Dir string
	// Driver selects a driver for a model; nil means subagent.StreamDriverFor.
	Driver func(subagent.Model, string) subagent.Driver
	// Store persists the backend session ids across restarts; nil keeps
	// them in memory only. Call Load after setting it.
	Store *Store

	mu       sync.Mutex
	sessions map[string]string // provider:name -> provider session id
}

// New returns a router whose provider CLIs run in dir.
func New(dir string) *Router { return &Router{Dir: dir} }

// ResolveSpec validates a model spec against spec/agent.yaml and returns its
// canonical provider:model:tier form.
func ResolveSpec(spec string) (string, error) {
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return "", err
	}
	for _, e := range subagent.KnownModelEntries() {
		if e.Model == m {
			return e.Spec, nil
		}
	}
	return m.String(), nil // e.g. a Pi model id outside spec/agent.yaml
}

// Session returns the backend session id the next turn of spec resumes.
func (r *Router) Session(spec string) string {
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sessions[sessionKey(m)]
}

// Load fills the session map from Store, so the first turn of each model
// resumes the backend session of an earlier run. A non-empty warning says a
// corrupt store was set aside. Without a Store it is a no-op.
func (r *Router) Load() (warning string, err error) {
	if r.Store == nil {
		return "", nil
	}
	sessions, warning, err := r.Store.Load()
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = sessions
	return warning, nil
}

// Turn runs prompt on the model named by spec and calls emit for every chunk,
// in order, from the calling goroutine. A ChunkSession chunk says whether the
// turn resumes the stored backend session or starts a new one. Cancelling ctx
// stops the provider process tree. Turn must not run concurrently for the
// same model.
func (r *Router) Turn(ctx context.Context, spec, prompt string, emit func(Chunk)) error {
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return err
	}
	pick := r.Driver
	if pick == nil {
		pick = subagent.StreamDriverFor
	}
	d := pick(m, r.Dir)
	if _, ok := d.(subagent.UnsupportedDriver); ok {
		return fmt.Errorf("provider %q is not supported by harnez tui yet: no driver (direct API clients are planned)", m.Provider)
	}
	key := sessionKey(m)
	id := r.Session(spec)
	if id != "" {
		emit(Chunk{Kind: ChunkSession, Text: SessionResumed})
	} else {
		emit(Chunk{Kind: ChunkSession, Text: SessionNew})
	}
	var storeErr error
	set := func(id string) {
		if err := r.setSession(key, id); err != nil && storeErr == nil {
			storeErr = err
		}
	}
	t := subagent.ResumeTurn{
		Driver: d,
		ID:     id,
		Opts:   subagent.RunOptions{Prompt: prompt, Model: m, Dir: r.Dir},
		// The backend session id is stored as soon as it is known, so a turn
		// cancelled after the provider reported it is still resumed next time.
		Events: func(ev subagent.Event) {
			switch ev.Kind {
			case "session":
				set(ev.Text)
			case "message":
				emit(Chunk{Kind: ChunkText, Text: ev.Text})
			case "activity":
				emit(Chunk{Kind: ChunkActivity, Text: ev.Text})
			}
		},
		// Only a verified stale session (subagent.IsStaleSession) starts
		// fresh; any other resume error is shown and the stored id is kept.
		OnFresh: func(note string) {
			set("")
			emit(Chunk{Kind: ChunkActivity, Text: note})
			emit(Chunk{Kind: ChunkSession, Text: SessionNew})
		},
	}
	if _, ok := d.(subagent.StreamingDriver); !ok {
		t.Events = nil
	}
	res, _, err := t.Do(ctx)
	if t.Events == nil && err == nil {
		for _, msg := range res.Messages {
			emit(Chunk{Kind: ChunkText, Text: msg})
		}
		if len(res.Messages) == 0 && res.Response != "" {
			emit(Chunk{Kind: ChunkText, Text: res.Response})
		}
	}
	if res != nil && res.SessionID != "" {
		set(res.SessionID)
	}
	if storeErr != nil {
		emit(Chunk{Kind: ChunkActivity, Text: "could not save backend session id: " + storeErr.Error()})
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// setSession updates the in-memory map and, with a Store, the session file.
func (r *Router) setSession(key, id string) error {
	r.mu.Lock()
	if r.sessions == nil {
		r.sessions = map[string]string{}
	}
	if r.sessions[key] == id {
		r.mu.Unlock()
		return nil
	}
	if id == "" {
		delete(r.sessions, key)
	} else {
		r.sessions[key] = id
	}
	r.mu.Unlock()
	if r.Store == nil {
		return nil
	}
	return r.Store.Set(key, id)
}

// sessionKey keeps one backend session per provider model; a tier change
// resumes the same session.
func sessionKey(m subagent.Model) string { return m.Provider + ":" + m.Name }
