package forgetracker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"ubunatic.com/harnez/internal/forge"
)

// FJLookPath and FJRun are package-level hooks for invoking the fj CLI in
// read-only fallback paths; tests replace them so unit tests never run real fj.
var (
	FJLookPath = exec.LookPath
	FJRun      = func(c Command) ([]byte, error) {
		cmd := exec.Command(c.Tool, c.Args...)
		return cmd.CombinedOutput()
	}
)

// FJSearchCommand builds a read-only fj issue search command for one state
// ("open" or "closed") using the host-qualified repo form (no --host flag).
func FJSearchCommand(t Target, state string) Command {
	return Command{
		Tool: "fj",
		Args: []string{"--style", "minimal", "issue", "search", "--repo", fjRepo(t), "--state", state},
	}
}

// ParseFJSearchOutput parses `fj --style minimal issue search` output lines of
// the form "#<N>: <title> (by <user>)", skipping count headers like "4 issues".
func ParseFJSearchOutput(t Target, state, out string) []forge.RemoteIssue {
	var list []forge.RemoteIssue
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 1 {
			continue
		}
		n, err := strconv.Atoi(line[1:colon])
		if err != nil || n <= 0 {
			continue
		}
		rest := strings.TrimSpace(line[colon+1:])
		if rest == "" {
			continue
		}
		title := rest
		var login string
		if idx := strings.LastIndex(rest, " (by "); idx >= 0 && strings.HasSuffix(rest, ")") {
			title = strings.TrimSpace(rest[:idx])
			login = strings.TrimSpace(rest[idx+len(" (by ") : len(rest)-1])
		}
		is := forge.RemoteIssue{
			Number:  n,
			Title:   title,
			State:   state,
			HTMLURL: fmt.Sprintf("https://%s/%s/%s/issues/%d", t.Forge.Host, t.Owner, t.Repo, n),
		}
		is.User.Login = login
		list = append(list, is)
	}
	return list
}

type fjFallbackSource struct {
	inner    Source
	hasToken func(forge.Forge) bool
	stderr   io.Writer
	lookPath func(string) (string, error)
	run      func(Command) ([]byte, error)
	mu       sync.Mutex
	noted    bool
}

// NewFJFallbackSource wraps inner so a Forgejo 401/404 with no read token falls
// back to read-only `fj issue search` (titles only).
func NewFJFallbackSource(inner Source, hasToken func(forge.Forge) bool, stderr io.Writer) Source {
	return &fjFallbackSource{
		inner:    inner,
		hasToken: hasToken,
		stderr:   stderr,
	}
}

// SetFallbackStderr updates the diagnostic writer on a fallback source, if src is one.
func SetFallbackStderr(src Source, w io.Writer) {
	if fb, ok := src.(*fjFallbackSource); ok {
		fb.mu.Lock()
		fb.stderr = w
		fb.mu.Unlock()
	}
}

func (s *fjFallbackSource) Issue(fg forge.Forge, owner, repo string, n int) (forge.RemoteIssue, error) {
	return s.inner.Issue(fg, owner, repo, n)
}

func (s *fjFallbackSource) Repo(fg forge.Forge, owner, repo string) error {
	return s.inner.Repo(fg, owner, repo)
}

func (s *fjFallbackSource) Issues(fg forge.Forge, owner, repo, state string, opts forge.ScanOptions) ([]forge.RemoteIssue, bool, error) {
	list, truncated, err := s.inner.Issues(fg, owner, repo, state, opts)
	if err == nil {
		return list, truncated, nil
	}
	if fg.Kind != "forgejo" {
		return nil, false, err
	}
	var apiErr *forge.APIError
	if !errors.As(err, &apiErr) || (apiErr.Status != 401 && apiErr.Status != 404) {
		return nil, false, err
	}
	if s.hasToken != nil && s.hasToken(fg) {
		return nil, false, err
	}
	lookPath := s.lookPath
	if lookPath == nil {
		lookPath = FJLookPath
	}
	if _, lookErr := lookPath("fj"); lookErr != nil {
		return nil, false, err
	}
	run := s.run
	if run == nil {
		run = FJRun
	}
	states := []string{state}
	if state == "" || state == "all" {
		states = []string{"open", "closed"}
	}
	t := Target{Forge: fg, Owner: owner, Repo: repo}
	var combined []forge.RemoteIssue
	for _, st := range states {
		out, runErr := run(FJSearchCommand(t, st))
		if runErr != nil {
			return nil, false, err
		}
		combined = append(combined, ParseFJSearchOutput(t, st, string(out))...)
	}
	s.noteOnce(t)
	return combined, false, nil
}

func (s *fjFallbackSource) noteOnce(t Target) {
	s.mu.Lock()
	if s.noted {
		s.mu.Unlock()
		return
	}
	s.noted = true
	w := s.stderr
	s.mu.Unlock()
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "note: using fj issue search for %s (titles only; no read token)\n", t)
}
