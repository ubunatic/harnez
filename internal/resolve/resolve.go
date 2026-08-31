// Package resolve implements the shared session_id and ticket_id
// resolution chain used by `harnez rate` (issue 117) and `harnez exec`
// (issue 118). See issues/121-multi-repo-session-and-ticket-id-resolution.md
// for the spec.
package resolve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionInactivityWindow is the sliding-window inactivity threshold used
// by the PPID/lock-file session fallback: consecutive harnez invocations
// within this window (from the same parent process) are grouped into the
// same session; a gap longer than this starts a new session.
const SessionInactivityWindow = 30 * time.Minute

// SessionEnvVars lists agent-provided session-id environment variables, in
// priority order (first non-empty wins).
//
// Verification for issue 121: only CLAUDE_CODE_SESSION_ID is confirmed —
// observed directly via `env` in a live Claude Code CLI session while
// developing this ticket (e.g. CLAUDE_CODE_SESSION_ID=6587aea8-...).
// CLAUDE_SESSION_ID, the name guessed in the original ticket 121 spec, is
// NOT set in that environment and is not otherwise referenced anywhere in
// this repo. ANTIGRAVITY_SESSION_ID and CODEX_SESSION_ID are UNCONFIRMED:
// no Antigravity or Codex session was available in this repo to inspect,
// and web search found no documented env var for either (a Codex CLI
// feature request for exactly this, openai/codex#8923, is open and
// unimplemented as of writing). They are kept here as placeholders so a
// confirmed name is a one-line change; do not rely on them without
// re-verifying first.
var SessionEnvVars = []string{
	"CLAUDE_CODE_SESSION_ID", // confirmed: Claude Code CLI (issue 121 verification)
	"CLAUDE_SESSION_ID",      // unconfirmed: original spec guess, not observed anywhere
	"ANTIGRAVITY_SESSION_ID", // unconfirmed: guess, no Antigravity session available to verify
	"CODEX_SESSION_ID",       // unconfirmed: guess, not documented/shipped by Codex CLI as of writing
}

// SessionOptions configures Session resolution.
type SessionOptions struct {
	// Explicit is a caller-supplied session_id override. If non-empty it
	// always wins and no implicit resolution happens.
	Explicit string
	// Getenv overrides os.Getenv, for tests. Optional.
	Getenv func(string) string
	// PPID overrides os.Getppid(), for tests. Optional.
	PPID int
	// LockDir overrides the sliding-window lock file directory
	// (default: DefaultStateDir()). Optional.
	LockDir string
	// Now overrides time.Now, for tests. Optional.
	Now func() time.Time
}

// Session resolves harnez's session_id: explicit override, then the first
// set agent-provided env var from SessionEnvVars, then a PPID-derived
// sliding-window lock file grouping same-parent-process invocations within
// SessionInactivityWindow into one session.
func Session(opts SessionOptions) (string, error) {
	if opts.Explicit != "" {
		return opts.Explicit, nil
	}

	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, name := range SessionEnvVars {
		if v := getenv(name); v != "" {
			return v, nil
		}
	}

	ppid := opts.PPID
	if ppid == 0 {
		ppid = os.Getppid()
	}
	lockDir := opts.LockDir
	if lockDir == "" {
		lockDir = DefaultStateDir()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return sessionFromLock(lockDir, ppid, now)
}

// sessionFromLock implements the PPID-derived, sliding-window lock file
// fallback tier. The lock file is keyed by a hash of the PPID (stable
// across calls from the same parent process) so repeated calls reuse the
// same file; its mtime tracks the sliding inactivity window, and its
// content holds the session ID minted for the current window.
func sessionFromLock(lockDir string, ppid int, now func() time.Time) (string, error) {
	key := shortHash(fmt.Sprintf("ppid:%d", ppid))
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", fmt.Errorf("resolve: creating session lock dir: %w", err)
	}
	lockPath := filepath.Join(lockDir, key+".lock")
	t := now()

	if fi, err := os.Stat(lockPath); err == nil && t.Sub(fi.ModTime()) <= SessionInactivityWindow {
		if data, err := os.ReadFile(lockPath); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				// Slide the window forward.
				_ = os.Chtimes(lockPath, t, t)
				return id, nil
			}
		}
	}

	// No lock file, or it expired: mint a new session for a new window.
	id := fmt.Sprintf("ppid-%s-%d", key, t.Unix())
	if err := os.WriteFile(lockPath, []byte(id), 0644); err != nil {
		return "", fmt.Errorf("resolve: writing session lock file: %w", err)
	}
	if err := os.Chtimes(lockPath, t, t); err != nil {
		return "", fmt.Errorf("resolve: setting session lock mtime: %w", err)
	}
	return id, nil
}

// TicketOptions configures Ticket resolution.
type TicketOptions struct {
	// Explicit is a caller-supplied ticket_id override. If non-empty it
	// always wins and no implicit resolution happens.
	Explicit string
	// SessionID is the already-resolved session_id (see Session), used to
	// look up and record the most-recently-used ticket_id. Optional; if
	// empty, inheritance is skipped.
	SessionID string
	// StateDir overrides where per-session ticket history is stored
	// (default: DefaultStateDir()). Optional.
	StateDir string
}

// Ticket resolves harnez's ticket_id ("<project_folder>/<ticket_name>"):
// explicit override (which, if SessionID is set, is also remembered as
// that session's most-recently-used ticket), else the most-recently-used
// ticket_id recorded for SessionID, else "" — an unresolved ticket_id is
// the normal, expected case, not an error.
//
// Earlier versions of this function also tried to infer a ticket from the
// current git repo's branch name (only if it "looked ticket-shaped") and
// treated a fully-unresolved ticket as a hard error. Removed per real-world
// usage feedback (2026-08-31, see issues/121's "Post-review correction"):
// this repo (and its user) never branches per ticket — every session works
// directly on the default branch — so the branch-name heuristic could
// never fire here and was dead weight elsewhere too; a session may also
// start outside any git repo at all (e.g. the parent projects/ directory)
// before `cd`-ing into one. The only signals resilient enough to assume
// are the working directory (already captured independently as
// ToolCall.ProjectName/WorkingDir on every row, regardless of ticket_id)
// and the session_id. Guessing a ticket_id from branch shape added
// fragility without a corresponding benefit, and hard-failing when nothing
// was inferable silently dropped entire tool_calls rows in `harnez exec`
// (issue 118's automatic capture) — see internal/telemetry callers, which
// must never lose a row over an unresolved ticket_id.
func Ticket(opts TicketOptions) (string, error) {
	stateDir := opts.StateDir
	if stateDir == "" {
		stateDir = DefaultStateDir()
	}

	if opts.Explicit != "" {
		if opts.SessionID != "" {
			if err := writeLastTicket(stateDir, opts.SessionID, opts.Explicit); err != nil {
				return "", fmt.Errorf("resolve: recording last ticket for session: %w", err)
			}
		}
		return opts.Explicit, nil
	}

	if opts.SessionID != "" {
		if last, ok := readLastTicket(stateDir, opts.SessionID); ok {
			return last, nil
		}
	}

	return "", nil
}

// DefaultStateDir returns ~/.harnez/sessions, where session lock files and
// per-session ticket history are stored.
func DefaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".harnez", "sessions")
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func ticketHistoryPath(stateDir, sessionID string) string {
	return filepath.Join(stateDir, shortHash(sessionID)+".ticket")
}

func readLastTicket(stateDir, sessionID string) (string, bool) {
	data, err := os.ReadFile(ticketHistoryPath(stateDir, sessionID))
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return "", false
	}
	return s, true
}

func writeLastTicket(stateDir, sessionID, ticketID string) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(ticketHistoryPath(stateDir, sessionID), []byte(ticketID), 0644)
}
