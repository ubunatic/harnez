package resolve_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/resolve"
)

// initRepo creates a bare-bones git repo layout (just enough for
// findRepoRoot/readBranch: a .git dir with a HEAD file) rooted at dir/name,
// checked out to branch. Returns the repo root and a nested subdirectory
// inside it.
func initRepo(t *testing.T, parent, name, branch string) (root, nested string) {
	t.Helper()
	root = filepath.Join(parent, name)
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	head := "ref: refs/heads/" + branch + "\n"
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(head), 0644); err != nil {
		t.Fatal(err)
	}
	nested = filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	return root, nested
}

func TestTicket_RepoRootFromNestedSubdir(t *testing.T) {
	tmp := t.TempDir()
	root, nested := initRepo(t, tmp, "myproject", "121-ticket-resolution")

	id, err := resolve.Ticket(resolve.TicketOptions{Dir: nested, StateDir: filepath.Join(tmp, "state")})
	if err != nil {
		t.Fatalf("Ticket() error = %v", err)
	}
	want := filepath.Base(root) + "/121-ticket-resolution"
	if id != want {
		t.Errorf("Ticket() = %q, want %q", id, want)
	}
}

func TestTicket_ExplicitOverrideWins(t *testing.T) {
	tmp := t.TempDir()
	_, nested := initRepo(t, tmp, "myproject", "121-ticket-resolution")

	id, err := resolve.Ticket(resolve.TicketOptions{
		Explicit: "other-project/999-explicit",
		Dir:      nested,
		StateDir: filepath.Join(tmp, "state"),
	})
	if err != nil {
		t.Fatalf("Ticket() error = %v", err)
	}
	if id != "other-project/999-explicit" {
		t.Errorf("Ticket() = %q, want explicit override", id)
	}
}

func TestTicket_NonTicketShapedBranchInheritsFromSession(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	sessionID := "sess-abc"

	// First: a ticket-shaped branch records a ticket for the session.
	_, nestedA := initRepo(t, tmp, "projA", "042-do-the-thing")
	first, err := resolve.Ticket(resolve.TicketOptions{Dir: nestedA, SessionID: sessionID, StateDir: stateDir})
	if err != nil {
		t.Fatalf("Ticket() first call error = %v", err)
	}
	if first != "projA/042-do-the-thing" {
		t.Fatalf("Ticket() first call = %q", first)
	}

	// Second: a repo on a non-ticket-shaped branch (e.g. "main") should
	// inherit the most recently used ticket_id from the session.
	_, nestedB := initRepo(t, tmp, "projB", "main")
	second, err := resolve.Ticket(resolve.TicketOptions{Dir: nestedB, SessionID: sessionID, StateDir: stateDir})
	if err != nil {
		t.Fatalf("Ticket() second call error = %v", err)
	}
	if second != first {
		t.Errorf("Ticket() second call = %q, want inherited %q", second, first)
	}
}

func TestSession_ExplicitOverrideWins(t *testing.T) {
	id, err := resolve.Session(resolve.SessionOptions{
		Explicit: "explicit-session",
		Getenv:   func(string) string { return "should-not-be-used" },
	})
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if id != "explicit-session" {
		t.Errorf("Session() = %q, want explicit override", id)
	}
}

func TestSession_EnvVarWins(t *testing.T) {
	env := map[string]string{"CLAUDE_CODE_SESSION_ID": "env-session-123"}
	id, err := resolve.Session(resolve.SessionOptions{
		Getenv: func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if id != "env-session-123" {
		t.Errorf("Session() = %q, want env var value", id)
	}
}

func TestSession_PPIDFallbackStableAcrossCalls(t *testing.T) {
	tmp := t.TempDir()
	getenv := func(string) string { return "" }
	now := func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

	opts := resolve.SessionOptions{Getenv: getenv, PPID: 4242, LockDir: tmp, Now: now}
	first, err := resolve.Session(opts)
	if err != nil {
		t.Fatalf("Session() first call error = %v", err)
	}
	second, err := resolve.Session(opts)
	if err != nil {
		t.Fatalf("Session() second call error = %v", err)
	}
	if first != second {
		t.Errorf("Session() not stable across calls: %q != %q", first, second)
	}

	// A different PPID must resolve to a different session.
	other, err := resolve.Session(resolve.SessionOptions{Getenv: getenv, PPID: 9999, LockDir: tmp, Now: now})
	if err != nil {
		t.Fatalf("Session() other-ppid call error = %v", err)
	}
	if other == first {
		t.Errorf("Session() for a different PPID unexpectedly matched: %q", other)
	}
}

func TestSession_SlidingWindowLockFile(t *testing.T) {
	tmp := t.TempDir()
	getenv := func(string) string { return "" }
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	nowAt := func(t time.Time) func() time.Time { return func() time.Time { return t } }

	// First call establishes a session at t=base.
	first, err := resolve.Session(resolve.SessionOptions{Getenv: getenv, PPID: 111, LockDir: tmp, Now: nowAt(base)})
	if err != nil {
		t.Fatalf("Session() first call error = %v", err)
	}

	// Within the 30-minute window: same session.
	within, err := resolve.Session(resolve.SessionOptions{
		Getenv: getenv, PPID: 111, LockDir: tmp, Now: nowAt(base.Add(10 * time.Minute)),
	})
	if err != nil {
		t.Fatalf("Session() within-window call error = %v", err)
	}
	if within != first {
		t.Errorf("Session() within window = %q, want same as first %q", within, first)
	}

	// The lock file's mtime should have been manipulated forward by the
	// within-window call (sliding). Force it further back in time,
	// simulating inactivity, by setting the mtime directly rather than
	// sleeping in the test.
	lockFiles, err := filepath.Glob(filepath.Join(tmp, "*.lock"))
	if err != nil || len(lockFiles) != 1 {
		t.Fatalf("expected exactly one lock file, got %v (err=%v)", lockFiles, err)
	}
	staleTime := base.Add(-1 * time.Hour)
	if err := os.Chtimes(lockFiles[0], staleTime, staleTime); err != nil {
		t.Fatalf("os.Chtimes() error = %v", err)
	}

	// More than 30 minutes after the (manipulated) lock mtime: new session.
	after, err := resolve.Session(resolve.SessionOptions{
		Getenv: getenv, PPID: 111, LockDir: tmp, Now: nowAt(base.Add(2 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("Session() after-window call error = %v", err)
	}
	if after == first {
		t.Errorf("Session() after inactivity window = %q, want a new session (was %q)", after, first)
	}
}
