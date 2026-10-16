package subagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type resumeFake struct {
	UnsupportedDriver
	calls     *[]string
	resumeErr error
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

// staleClaude is the verified claude CLI error for an unknown --resume id.
func staleClaude(id string) error {
	return fmt.Errorf("claude resume: %w", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "No conversation found with session ID: " + id})
}

func TestIsStaleSession(t *testing.T) {
	if !IsStaleSession("claude", staleClaude("abc")) {
		t.Error("claude stderr 'No conversation found' must be stale")
	}
	codexStale := &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "Error: thread/resume: thread/resume failed: no rollout found for thread id abc (code -32600)"}
	if !IsStaleSession("codex", fmt.Errorf("codex resume: %w", codexStale)) {
		t.Error("codex stderr 'no rollout found for thread id' must be stale")
	}
	for name, tc := range map[string]struct {
		provider string
		err      error
	}{
		"nil":                 {"claude", nil},
		"plain error":         {"claude", errors.New("No conversation found with session ID: abc")},
		"model output":        {"claude", errors.New("claude: result: the agent said No conversation found with session ID: x")},
		"mid-line stderr":     {"claude", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "warn: No conversation found with session ID: x"}},
		"quota":               {"claude", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "usage limit reached"}},
		"unverified provider": {"pi", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "No conversation found with session ID: x"}},
		"codex other":         {"codex", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "session not found"}},
		"codex quota":         {"codex", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "Error: usage limit reached"}},
	} {
		if IsStaleSession(tc.provider, tc.err) {
			t.Errorf("%s: IsStaleSession = true", name)
		}
	}
}

func TestResumeTurnResumesNatively(t *testing.T) {
	var calls []string
	res, resumed, err := ResumeTurn{Driver: resumeFake{calls: &calls}, ID: "s1", Opts: RunOptions{Prompt: "p", Model: Model{Provider: "claude"}}}.Do(context.Background())
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
		Driver:  resumeFake{calls: &calls, resumeErr: staleClaude("s1")},
		ID:      "s1",
		Opts:    RunOptions{Prompt: "p", Model: Model{Provider: "claude"}},
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

// Non-stale errors (quota, auth, unverified providers) keep failing so
// callers keep the session and their bookkeeping.
func TestResumeTurnNonStaleErrorsFail(t *testing.T) {
	for _, tc := range []struct {
		provider string
		err      error
	}{
		{"claude", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "usage limit reached"}},
		{"codex", &ProviderExitError{Err: errors.New("exit status 1"), Stderr: "No conversation found with session ID: s1"}},
	} {
		var calls []string
		_, resumed, err := ResumeTurn{Driver: resumeFake{calls: &calls, resumeErr: tc.err}, ID: "s1", Opts: RunOptions{Model: Model{Provider: tc.provider}}}.Do(context.Background())
		if err == nil || !resumed || len(calls) != 1 {
			t.Fatalf("%s: err=%v resumed=%v calls=%v", tc.provider, err, resumed, calls)
		}
	}
}
