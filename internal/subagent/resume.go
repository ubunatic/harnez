package subagent

import (
	"context"
	"errors"
	"strings"
)

// IsStaleSession reports whether err says the provider session to resume is
// gone, so a fresh session is the only way forward. It matches only the
// provider CLI's own stderr (ProviderExitError), never model output, against
// the verified line prefixes in stale_session_errors of spec/agent.yaml. A
// provider without entries is never stale: false negatives keep the stored
// session, false positives would drop it.
func IsStaleSession(provider string, err error) bool {
	var exit *ProviderExitError
	if !errors.As(err, &exit) {
		return false
	}
	spec, specErr := agentSpecOnce()
	if specErr != nil {
		return false
	}
	return matchStaleStderr(spec.StaleSessionErrors[provider], exit.Stderr)
}

func matchStaleStderr(prefixes []string, stderr string) bool {
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		for _, p := range prefixes {
			if p != "" && strings.HasPrefix(line, p) {
				return true
			}
		}
	}
	return false
}

// ResumeTurn runs one provider turn shared by harnez tui and harnez agent
// resume: a native resume of ID (no history replay) when ID is set, else a new
// session. A resume that fails on a stale session starts a new session with
// the same options; any other error is returned and the caller keeps ID.
type ResumeTurn struct {
	Driver Driver
	// ID is the provider session to resume; empty starts a new session.
	ID   string
	Opts RunOptions
	// Events receives live events of a StreamingDriver; nil uses the batch
	// Run/Resume calls even when the driver streams.
	Events func(Event)
	// OnFresh is called with a short note before the new session starts
	// after a stale resume.
	OnFresh func(note string)
}

// Do runs the turn. resumed reports whether the result came from the resumed
// session; false means a new session (check TurnResult.SessionID).
func (t ResumeTurn) Do(ctx context.Context) (res *TurnResult, resumed bool, err error) {
	if t.ID != "" {
		res, err = t.call(ctx, t.ID)
		if err == nil || ctx.Err() != nil || !IsStaleSession(t.Opts.Model.Provider, err) {
			return res, true, err
		}
		if t.OnFresh != nil {
			t.OnFresh("could not resume session " + t.ID + " (" + firstErrLine(err) + "); starting a new session")
		}
	}
	res, err = t.call(ctx, "")
	return res, false, err
}

func (t ResumeTurn) call(ctx context.Context, id string) (*TurnResult, error) {
	if sd, ok := t.Driver.(StreamingDriver); ok && t.Events != nil {
		if id == "" {
			return sd.RunStream(ctx, t.Opts, t.Events)
		}
		return sd.ResumeStream(ctx, id, t.Opts.Prompt, t.Opts.Model, t.Events)
	}
	if id == "" {
		return t.Driver.Run(ctx, t.Opts)
	}
	return t.Driver.Resume(ctx, id, t.Opts.Prompt, t.Opts.Model)
}

func firstErrLine(err error) string {
	line, _, _ := strings.Cut(err.Error(), "\n")
	return line
}
