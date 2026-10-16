package subagent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type resumeFake struct {
	UnsupportedDriver
	calls     *[]string
	resumeErr error
	resumable bool
}

func (d resumeFake) Run(_ context.Context, o RunOptions) (*TurnResult, error) {
	*d.calls = append(*d.calls, "run "+o.Prompt)
	return &TurnResult{SessionID: "fresh"}, nil
}

func (d resumeFake) Resume(_ context.Context, id, prompt string, _ Model) (*TurnResult, error) {
	*d.calls = append(*d.calls, "resume "+id+" "+prompt)
	if d.resumeErr != nil {
		return nil, d.resumeErr
	}
	return &TurnResult{SessionID: id}, nil
}

type checkedFake struct{ resumeFake }

func (d checkedFake) CheckResumable(string) (bool, string) {
	return d.resumable, "Codex session file is missing"
}

func TestIsStaleSession(t *testing.T) {
	for _, msg := range []string{
		"claude resume: exit status 1: No conversation found with session ID: abc",
		"codex resume: thread/resume failed: no rollout found for thread id abc",
		"pi resume: Session not found: abc",
	} {
		if !IsStaleSession(errors.New(msg)) {
			t.Errorf("IsStaleSession(%q) = false", msg)
		}
	}
	for _, err := range []error{nil, errors.New("usage limit reached"), errors.New("401 unauthorized"), context.Canceled} {
		if IsStaleSession(err) {
			t.Errorf("IsStaleSession(%v) = true", err)
		}
	}
}

func TestResumeTurnResumesNatively(t *testing.T) {
	var calls []string
	res, resumed, err := ResumeTurn{Driver: resumeFake{calls: &calls}, ID: "s1", Opts: RunOptions{Prompt: "p"}}.Do(context.Background())
	if err != nil || !resumed || res.SessionID != "s1" {
		t.Fatalf("Do = %v, %v, %v", res, resumed, err)
	}
	if s := strings.Join(calls, "|"); s != "resume s1 p" {
		t.Fatalf("calls = %q", s)
	}
}

func TestResumeTurnStaleStartsFresh(t *testing.T) {
	var calls []string
	var note string
	res, resumed, err := ResumeTurn{
		Driver:  resumeFake{calls: &calls, resumeErr: errors.New("No conversation found with session ID: s1")},
		ID:      "s1",
		Opts:    RunOptions{Prompt: "p"},
		OnFresh: func(n string) { note = n },
	}.Do(context.Background())
	if err != nil || resumed || res.SessionID != "fresh" {
		t.Fatalf("Do = %v, %v, %v", res, resumed, err)
	}
	if s := strings.Join(calls, "|"); s != "resume s1 p|run p" {
		t.Fatalf("calls = %q", s)
	}
	if !strings.Contains(note, "could not resume session s1") || !strings.Contains(note, "starting a new session") {
		t.Fatalf("note = %q", note)
	}
}

// Non-stale errors (quota, auth) keep failing so callers keep their bookkeeping.
func TestResumeTurnDefaultRetryKeepsOtherErrors(t *testing.T) {
	var calls []string
	_, resumed, err := ResumeTurn{Driver: resumeFake{calls: &calls, resumeErr: errors.New("usage limit reached")}, ID: "s1"}.Do(context.Background())
	if err == nil || !resumed {
		t.Fatalf("Do err = %v, resumed = %v", err, resumed)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v", calls)
	}
}

// A ResumeChecker that knows the session is gone skips the doomed resume.
func TestResumeTurnCheckerSkipsResume(t *testing.T) {
	var calls []string
	var note string
	_, resumed, err := ResumeTurn{Driver: checkedFake{resumeFake{calls: &calls}}, ID: "s1", Opts: RunOptions{Prompt: "p"}, OnFresh: func(n string) { note = n }}.Do(context.Background())
	if err != nil || resumed {
		t.Fatalf("Do err = %v, resumed = %v", err, resumed)
	}
	if s := strings.Join(calls, "|"); s != "run p" {
		t.Fatalf("calls = %q", s)
	}
	if !strings.Contains(note, "Codex session file is missing") {
		t.Fatalf("note = %q", note)
	}
}
