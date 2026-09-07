package resolve_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/resolve"
)

// TestTicket_UnresolvedIsEmptyNotError is the regression check for the
// 2026-08-31 simplification: no explicit ticket and no session history
// must resolve to ("", nil), never an error. The prior branch-name-based
// heuristic (removed) meant this exact case previously errored for any
// repo working directly on its default branch — this repo's own usage
// pattern — silently dropping every harnez exec telemetry row.
func TestTicket_UnresolvedIsEmptyNotError(t *testing.T) {
	tmp := t.TempDir()
	id, err := resolve.Ticket(resolve.TicketOptions{StateDir: filepath.Join(tmp, "state")})
	if err != nil {
		t.Fatalf("Ticket() error = %v, want nil (unresolved is not an error)", err)
	}
	if id != "" {
		t.Errorf("Ticket() = %q, want empty string", id)
	}
}

func TestTicket_ExplicitOverrideWins(t *testing.T) {
	tmp := t.TempDir()
	id, err := resolve.Ticket(resolve.TicketOptions{
		Explicit: "other-project/999-explicit",
		StateDir: filepath.Join(tmp, "state"),
	})
	if err != nil {
		t.Fatalf("Ticket() error = %v", err)
	}
	if id != "other-project/999-explicit" {
		t.Errorf("Ticket() = %q, want explicit override", id)
	}
}

// TestTicket_ExplicitPersistsAndIsInheritedBySession confirms the only
// remaining implicit-resolution path: an explicit ticket recorded once for
// a session is inherited by later calls in the same session that don't
// pass one, e.g. harnez rate setting a ticket explicitly once, then
// harnez exec's automatic per-Bash-call capture inheriting it for the
// rest of the session without needing it repeated.
func TestTicket_ExplicitPersistsAndIsInheritedBySession(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	sessionID := "sess-abc"

	first, err := resolve.Ticket(resolve.TicketOptions{
		Explicit: "harnez/124-post-tool-use", SessionID: sessionID, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("Ticket() first call error = %v", err)
	}

	second, err := resolve.Ticket(resolve.TicketOptions{SessionID: sessionID, StateDir: stateDir})
	if err != nil {
		t.Fatalf("Ticket() second call error = %v", err)
	}
	if second != first {
		t.Errorf("Ticket() second call = %q, want inherited %q", second, first)
	}
}

// TestTicket_DifferentSessionDoesNotInherit confirms inheritance is scoped
// per session_id, not global.
func TestTicket_DifferentSessionDoesNotInherit(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")

	if _, err := resolve.Ticket(resolve.TicketOptions{
		Explicit: "harnez/124-post-tool-use", SessionID: "sess-A", StateDir: stateDir,
	}); err != nil {
		t.Fatalf("Ticket() sess-A call error = %v", err)
	}

	id, err := resolve.Ticket(resolve.TicketOptions{SessionID: "sess-B", StateDir: stateDir})
	if err != nil {
		t.Fatalf("Ticket() sess-B call error = %v", err)
	}
	if id != "" {
		t.Errorf("Ticket() for an unrelated session = %q, want empty (no cross-session leak)", id)
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

func TestSession_AntigravityConversationID(t *testing.T) {
	env := map[string]string{
		"ANTIGRAVITY_CONVERSATION_ID": "8dfe521d-1918-497d-a548-fb2394b49f53",
		"ANTIGRAVITY_SESSION_ID":      "fallback-session-id",
	}
	id, err := resolve.Session(resolve.SessionOptions{
		Getenv: func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if id != "8dfe521d-1918-497d-a548-fb2394b49f53" {
		t.Errorf("Session() = %q, want ANTIGRAVITY_CONVERSATION_ID value", id)
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
