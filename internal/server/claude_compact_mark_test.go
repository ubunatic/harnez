package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Issue 1303 M1d: the PreCompact mark round-trips per session, lives under the
// state dir, and refuses a session id that could leave it.
func TestClaudeCompactMarkRoundTripsAndStaysInStateDir(t *testing.T) {
	isolateCompactEnv(t)
	at := time.Now().UTC().Truncate(time.Millisecond)
	if err := writeClaudeCompactMark("mark-session", at); err != nil {
		t.Fatal(err)
	}
	if got := readClaudeCompactMark("mark-session"); !got.Equal(at) {
		t.Errorf("mark = %v, want %v", got, at)
	}
	if got := readClaudeCompactMark("no-such-session"); !got.IsZero() {
		t.Errorf("missing mark = %v, want zero time", got)
	}
	path, err := claudeCompactMarkPath("mark-session")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, os.Getenv("XDG_STATE_HOME")) {
		t.Errorf("mark path %q is outside the state dir", path)
	}
}

func TestClaudeCompactMarkRefusesUnsafeSessionID(t *testing.T) {
	isolateCompactEnv(t)
	for _, id := range []string{"", ".", "..", "../escape", "a/b", filepath.Join("x", "y")} {
		if _, err := claudeCompactMarkPath(id); err == nil {
			t.Errorf("session id %q accepted", id)
		}
		if err := writeClaudeCompactMark(id, time.Now()); err == nil {
			t.Errorf("write for session id %q succeeded", id)
		}
		if got := readClaudeCompactMark(id); !got.IsZero() {
			t.Errorf("read for session id %q = %v, want zero", id, got)
		}
	}
}
