// Package router runs harnez tui turns through the existing subagent drivers.
//
// Subscription-bound providers (claude, codex, agy, pi) go through their CLI
// driver so a turn draws on the user's plan quota. Drivers that implement
// subagent.StreamingDriver (codex) report messages and activity while the
// turn runs; batch drivers report the turn's messages when it completes.
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
)

// Chunk is one piece of turn output, delivered in arrival order.
type Chunk struct {
	Kind ChunkKind
	Text string
}

// Router routes prompts to the driver of the active model and keeps one
// backend session per provider model, so the next prompt resumes it.
type Router struct {
	// Dir is the working directory the provider CLIs run in.
	Dir string
	// Driver selects a driver for a model; nil means subagent.DriverFor.
	Driver func(subagent.Model, string) subagent.Driver

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

// Turn runs prompt on the model named by spec and calls emit for every chunk,
// in order, from the calling goroutine. Cancelling ctx stops the provider
// process tree. Turn must not run concurrently for the same model.
func (r *Router) Turn(ctx context.Context, spec, prompt string, emit func(Chunk)) error {
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return err
	}
	pick := r.Driver
	if pick == nil {
		pick = subagent.DriverFor
	}
	d := pick(m, r.Dir)
	if _, ok := d.(subagent.UnsupportedDriver); ok {
		return fmt.Errorf("provider %q is not supported by harnez tui yet: no driver (direct API clients are planned)", m.Provider)
	}
	key := sessionKey(m)
	r.mu.Lock()
	id := r.sessions[key]
	r.mu.Unlock()

	var res *subagent.TurnResult
	if sd, ok := d.(subagent.StreamingDriver); ok {
		fn := func(ev subagent.Event) {
			switch ev.Kind {
			case "message":
				emit(Chunk{Kind: ChunkText, Text: ev.Text})
			case "activity":
				emit(Chunk{Kind: ChunkActivity, Text: ev.Text})
			}
		}
		if id == "" {
			res, err = sd.RunStream(ctx, subagent.RunOptions{Prompt: prompt, Model: m, Dir: r.Dir}, fn)
		} else {
			res, err = sd.ResumeStream(ctx, id, prompt, m, fn)
		}
	} else {
		if id == "" {
			res, err = d.Run(ctx, subagent.RunOptions{Prompt: prompt, Model: m, Dir: r.Dir})
		} else {
			res, err = d.Resume(ctx, id, prompt, m)
		}
		if err == nil {
			for _, msg := range res.Messages {
				emit(Chunk{Kind: ChunkText, Text: msg})
			}
			if len(res.Messages) == 0 && res.Response != "" {
				emit(Chunk{Kind: ChunkText, Text: res.Response})
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if res.SessionID != "" {
		r.mu.Lock()
		if r.sessions == nil {
			r.sessions = map[string]string{}
		}
		r.sessions[key] = res.SessionID
		r.mu.Unlock()
	}
	return nil
}

// sessionKey keeps one backend session per provider model; a tier change
// resumes the same session.
func sessionKey(m subagent.Model) string { return m.Provider + ":" + m.Name }
