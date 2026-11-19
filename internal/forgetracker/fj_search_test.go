package forgetracker

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/forge"
)

type errSource struct {
	issueErr  error
	issuesErr error
	repoErr   error
}

func (e *errSource) Issue(forge.Forge, string, string, int) (forge.RemoteIssue, error) {
	return forge.RemoteIssue{}, e.issueErr
}

func (e *errSource) Issues(forge.Forge, string, string, string, forge.ScanOptions) ([]forge.RemoteIssue, bool, error) {
	return nil, false, e.issuesErr
}

func (e *errSource) Repo(forge.Forge, string, string) error { return e.repoErr }

func TestFJSearchCommandHasNoHostFlag(t *testing.T) {
	c := FJSearchCommand(cbTarget, "open")
	want := []string{"--style", "minimal", "issue", "search", "--repo", "codeberg.org/o/r", "--state", "open"}
	if c.Tool != "fj" || !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("got %s %q, want fj %q", c.Tool, c.Args, want)
	}
	for _, arg := range c.Args {
		if arg == "--host" || arg == "-H" {
			t.Fatalf("unexpected host flag in %q", c.Args)
		}
	}
}

func TestParseFJSearchOutput(t *testing.T) {
	raw := strings.Join([]string{
		"3 issues",
		"#295: Fix login redirect (by alice)",
		"#12: Handle (nested) parens in title (by bob-1)",
		"",
		"No other lines",
		"#7: Title without author suffix",
	}, "\n")
	got := ParseFJSearchOutput(cbTarget, "open", raw)
	if len(got) != 3 {
		t.Fatalf("got %d issues, want 3: %+v", len(got), got)
	}
	if got[0].Number != 295 || got[0].Title != "Fix login redirect" || got[0].State != "open" || got[0].Body != "" ||
		got[0].HTMLURL != "https://codeberg.org/o/r/issues/295" || got[0].User.Login != "alice" {
		t.Fatalf("issue 0 mismatch: %+v", got[0])
	}
	if got[1].Number != 12 || got[1].Title != "Handle (nested) parens in title" || got[1].User.Login != "bob-1" {
		t.Fatalf("issue 1 mismatch: %+v", got[1])
	}
	if got[2].Number != 7 || got[2].Title != "Title without author suffix" || got[2].User.Login != "" {
		t.Fatalf("issue 2 mismatch: %+v", got[2])
	}
}

func TestFJFallbackSourceQueriesStatesAndNotesOnce(t *testing.T) {
	oldLook, oldRun := FJLookPath, FJRun
	t.Cleanup(func() { FJLookPath, FJRun = oldLook, oldRun })

	FJLookPath = lookOnly("fj")
	var calls []Command
	FJRun = func(c Command) ([]byte, error) {
		calls = append(calls, c)
		for i := 0; i < len(c.Args)-1; i++ {
			if c.Args[i] == "--state" {
				if c.Args[i+1] == "open" {
					return []byte("1 issue\n#10: Open bug (by alice)\n"), nil
				}
				return []byte("1 issue\n#4: Closed bug (by bob)\n"), nil
			}
		}
		return nil, errors.New("missing --state")
	}

	var stderr bytes.Buffer
	inner := &errSource{issuesErr: &forge.APIError{Status: 404, Detail: "not found"}}
	src := NewFJFallbackSource(inner, func(forge.Forge) bool { return false }, &stderr)

	got, truncated, err := src.Issues(cbTarget.Forge, cbTarget.Owner, cbTarget.Repo, "all", forge.ScanOptions{})
	if err != nil || truncated || len(got) != 2 {
		t.Fatalf("all fallback: len=%d truncated=%v err=%v", len(got), truncated, err)
	}
	if got[0].Number != 10 || got[0].State != "open" || got[1].Number != 4 || got[1].State != "closed" {
		t.Fatalf("unexpected issues: %+v", got)
	}
	if len(calls) != 2 {
		t.Fatalf("want 2 fj calls for state=all, got %d", len(calls))
	}
	wantNote := "note: using fj issue search for codeberg.org/o/r (titles only; no read token)\n"
	if stderr.String() != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantNote)
	}

	// Single-state query calls fj once and does not repeat the note.
	calls = nil
	got, _, err = src.Issues(cbTarget.Forge, cbTarget.Owner, cbTarget.Repo, "closed", forge.ScanOptions{})
	if err != nil || len(got) != 1 || got[0].Number != 4 || got[0].State != "closed" || len(calls) != 1 {
		t.Fatalf("closed fallback: got=%+v calls=%d err=%v", got, len(calls), err)
	}
	if strings.Count(stderr.String(), "note: using fj issue search") != 1 {
		t.Fatalf("expected note once, got %q", stderr.String())
	}
}

func TestFJFallbackSourceGates(t *testing.T) {
	oldLook, oldRun := FJLookPath, FJRun
	t.Cleanup(func() { FJLookPath, FJRun = oldLook, oldRun })

	fjCalled := false
	FJLookPath = lookOnly("fj")
	FJRun = func(c Command) ([]byte, error) {
		fjCalled = true
		return []byte("1 issue\n#1: T (by a)\n"), nil
	}

	t.Run("skips when read token is configured", func(t *testing.T) {
		fjCalled = false
		src := NewFJFallbackSource(&errSource{issuesErr: &forge.APIError{Status: 404, Detail: "404"}}, func(forge.Forge) bool { return true }, &bytes.Buffer{})
		_, _, err := src.Issues(cbTarget.Forge, cbTarget.Owner, cbTarget.Repo, "open", forge.ScanOptions{})
		if err == nil || fjCalled {
			t.Fatalf("expected API error without fj call, err=%v fjCalled=%v", err, fjCalled)
		}
	})

	t.Run("skips non-forgejo forge", func(t *testing.T) {
		fjCalled = false
		src := NewFJFallbackSource(&errSource{issuesErr: &forge.APIError{Status: 404, Detail: "404"}}, func(forge.Forge) bool { return false }, &bytes.Buffer{})
		_, _, err := src.Issues(ghTarget.Forge, ghTarget.Owner, ghTarget.Repo, "open", forge.ScanOptions{})
		if err == nil || fjCalled {
			t.Fatalf("expected API error without fj call on GitHub, err=%v fjCalled=%v", err, fjCalled)
		}
	})

	t.Run("skips non-401/404 status", func(t *testing.T) {
		fjCalled = false
		src := NewFJFallbackSource(&errSource{issuesErr: &forge.APIError{Status: 500, Detail: "boom"}}, func(forge.Forge) bool { return false }, &bytes.Buffer{})
		_, _, err := src.Issues(cbTarget.Forge, cbTarget.Owner, cbTarget.Repo, "open", forge.ScanOptions{})
		if err == nil || fjCalled {
			t.Fatalf("expected API error without fj call on 500, err=%v fjCalled=%v", err, fjCalled)
		}
	})

	t.Run("returns original APIError when fj is missing or fails", func(t *testing.T) {
		FJLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
		var stderr bytes.Buffer
		src := NewFJFallbackSource(&errSource{issuesErr: &forge.APIError{Status: 401, Detail: "unauth"}}, func(forge.Forge) bool { return false }, &stderr)
		_, _, err := src.Issues(cbTarget.Forge, cbTarget.Owner, cbTarget.Repo, "open", forge.ScanOptions{})
		var apiErr *forge.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != 401 || stderr.Len() != 0 {
			t.Fatalf("missing fj: err=%v stderr=%q", err, stderr.String())
		}

		FJLookPath = lookOnly("fj")
		FJRun = func(Command) ([]byte, error) { return []byte("not logged in"), errors.New("exit 1") }
		_, _, err = src.Issues(cbTarget.Forge, cbTarget.Owner, cbTarget.Repo, "open", forge.ScanOptions{})
		if !errors.As(err, &apiErr) || apiErr.Status != 401 || stderr.Len() != 0 {
			t.Fatalf("failing fj: err=%v stderr=%q", err, stderr.String())
		}
	})
}
