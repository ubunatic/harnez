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
	"regexp"
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
	// Dir is the directory to resolve the repo root/branch from
	// (default: os.Getwd()). Optional.
	Dir string
	// SessionID is the already-resolved session_id (see Session), used to
	// look up and record the most-recently-used ticket_id when the current
	// branch isn't ticket-shaped. Optional; if empty, inheritance is
	// skipped.
	SessionID string
	// StateDir overrides where per-session ticket history is stored
	// (default: DefaultStateDir()). Optional.
	StateDir string
}

// Ticket resolves harnez's ticket_id ("<project_folder>/<ticket_name>"):
// explicit override, then the nearest git repo root's directory name as
// project_folder with the active branch name as ticket_name if the branch
// looks ticket-shaped, else the most-recently-used ticket_id recorded for
// SessionID.
func Ticket(opts TicketOptions) (string, error) {
	if opts.Explicit != "" {
		return opts.Explicit, nil
	}

	dir := opts.Dir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve: getting working directory: %w", err)
		}
		dir = wd
	}
	stateDir := opts.StateDir
	if stateDir == "" {
		stateDir = DefaultStateDir()
	}

	var ticketID string
	if root, gitDir, ok := findRepoRoot(dir); ok {
		if branch, ok := readBranch(gitDir); ok && isTicketShaped(branch) {
			ticketID = filepath.Base(root) + "/" + branch
		}
	}

	if ticketID == "" && opts.SessionID != "" {
		if last, ok := readLastTicket(stateDir, opts.SessionID); ok {
			ticketID = last
		}
	}

	if ticketID == "" {
		return "", fmt.Errorf("resolve: could not determine ticket_id: %s is not on a ticket-shaped branch and no prior ticket was recorded for this session", dir)
	}

	if opts.SessionID != "" {
		if err := writeLastTicket(stateDir, opts.SessionID, ticketID); err != nil {
			return "", fmt.Errorf("resolve: recording last ticket for session: %w", err)
		}
	}
	return ticketID, nil
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

// ticketShapedRe matches branch names that look like a ticket ID: an
// optional single-segment prefix (e.g. "issue/", "feature/") followed by a
// leading number and a slug, mirroring this repo's own issues/NNN-slug.md
// convention.
var ticketShapedRe = regexp.MustCompile(`^(?:[a-zA-Z][a-zA-Z0-9_.-]*/)?[0-9]+[-_][a-zA-Z0-9-]+$`)

func isTicketShaped(branch string) bool {
	return branch != "" && ticketShapedRe.MatchString(branch)
}

// findRepoRoot searches upward from startDir for a .git entry (directory
// or worktree/submodule gitdir file). It returns the directory containing
// that entry as root, and the actual git directory (resolved through a
// "gitdir:" indirection file, if present) as gitDir.
func findRepoRoot(startDir string) (root, gitDir string, ok bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		dir = startDir
	}
	for {
		gitPath := filepath.Join(dir, ".git")
		if fi, statErr := os.Stat(gitPath); statErr == nil {
			if fi.IsDir() {
				return dir, gitPath, true
			}
			if data, readErr := os.ReadFile(gitPath); readErr == nil {
				content := strings.TrimSpace(string(data))
				if target, cut := strings.CutPrefix(content, "gitdir:"); cut {
					target = strings.TrimSpace(target)
					if !filepath.IsAbs(target) {
						target = filepath.Join(dir, target)
					}
					return dir, filepath.Clean(target), true
				}
			}
			return dir, gitPath, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", "", false
}

// readBranch reads the active branch name from gitDir/HEAD. It returns
// ok=false for a detached HEAD (no branch).
func readBranch(gitDir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false
	}
	content := strings.TrimSpace(string(data))
	if name, ok := strings.CutPrefix(content, "ref: refs/heads/"); ok {
		return name, true
	}
	return "", false
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
