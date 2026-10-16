package subagent

import (
	"context"
	"strings"
)

// staleSessionMarkers are lower-case fragments of provider CLI errors for a
// session id the provider no longer knows (deleted, expired, other machine).
var staleSessionMarkers = []string{
	"no conversation found",   // claude -p --resume <unknown id>
	"conversation not found",  // agy --conversation
	"session not found",       // pi, generic
	"no rollout found",        // codex exec resume
	"thread not found",        // codex app-server wording
	"session file is missing", // CodexDriver.CheckResumable
}

// IsStaleSession reports whether err says the provider session to resume is
// gone, so a fresh session is the only way forward. Quota, auth and runtime
// errors are not stale.
func IsStaleSession(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, m := range staleSessionMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

// ResumeTurn runs one provider turn shared by harnez tui and harnez agent
// resume: a native resume of ID (no history replay) when ID is set, else a new
// session. A resume that fails on a stale session starts a new session with
// the same options instead of failing every later prompt.
type ResumeTurn struct {
	Driver Driver
	// ID is the provider session to resume; empty starts a new session.
	ID   string
	Opts RunOptions
	// Events receives live events of a StreamingDriver; nil uses the batch
	// Run/Resume calls even when the driver streams.
	Events func(Event)
	// Retry reports whether a resume error starts a new session; nil means
	// IsStaleSession. A cancelled context never retries.
	Retry func(error) bool
	// OnFresh is called with a short note before the new session starts
	// after a failed or impossible resume.
	OnFresh func(note string)
}

// Do runs the turn. resumed reports whether the result came from the resumed
// session; false means a new session (check TurnResult.SessionID).
func (t ResumeTurn) Do(ctx context.Context) (res *TurnResult, resumed bool, err error) {
	if t.ID != "" {
		note := ""
		if c, ok := t.Driver.(ResumeChecker); ok {
			if ok, reason := c.CheckResumable(t.ID); !ok {
				note = "session " + t.ID + " cannot be resumed (" + reason + "); starting a new session"
			}
		}
		if note == "" {
			res, err = t.call(ctx, t.ID)
			retry := t.Retry
			if retry == nil {
				retry = IsStaleSession
			}
			if err == nil || ctx.Err() != nil || !retry(err) {
				return res, true, err
			}
			note = "could not resume session " + t.ID + " (" + firstErrLine(err) + "); starting a new session"
		}
		if t.OnFresh != nil {
			t.OnFresh(note)
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
