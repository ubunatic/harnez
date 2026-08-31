# 121 — Multi-repo session & ticket ID resolution strategy

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[117-harnez-rate-command]], [[118-harnez-exec-shell-interceptor]], `docs/session-cwd` work in [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]]

## Problem

Agents switch working directories across multiple git repos within one
session. [[117]] and [[118]] both need `session_id` and `ticket_id`
resolved correctly regardless of which repo the current shell/tool call
happens to be in, without the agent having to pass both explicitly on
every call.

## Scope

Implement the resolution chain as its own package (used by both 117 and
118, not duplicated):

**Ticket ID** (`<project_folder>/<ticket_name>`):
1. Explicit: caller passed it directly (`harnez rate ... <ticket_id>` or
   `harnez exec --ticket ...`) — always wins, no resolution needed.
2. Implicit fallback: nearest repository root's directory name as
   `project_folder`; `ticket_name` from the active git branch name if it
   looks ticket-shaped, else inherit the most recently used ticket ID
   within the same resolved session (see session resolution below).

**Session ID**:
1. Primary: agent-provided env var (`CLAUDE_SESSION_ID`,
   `ANTIGRAVITY_SESSION_ID`, or equivalent Codex var if one exists —
   confirm actual var names per agent before hardcoding, they may not
   match this spec's guessed names).
2. Secondary: PPID-derived hash when no env var is present.
3. Fallback: sliding-window lock file at
   `~/.harnez/sessions/<hash>.lock`, grouping commands within a 30-
   minute inactivity threshold into the same session.

## Acceptance Criteria

- [ ] Repo-root detection works correctly when invoked from a nested
      subdirectory of a git repo (not just the root).
- [ ] Explicit `ticket_id`/`session_id` always overrides implicit
      resolution — covered by a test that supplies both and checks the
      explicit value wins.
- [ ] PPID-hash fallback produces a stable session ID across multiple
      `harnez rate`/`harnez exec` calls from the same parent process.
- [ ] Sliding-window lock file: two calls more than 30 minutes apart
      resolve to different session IDs; two calls within the window
      resolve to the same one — covered by a test that manipulates the
      lock file's mtime rather than sleeping 30 minutes.
- [ ] Actual env var names for each agent are confirmed (not assumed)
      before this ticket is marked resolved — note findings here if any
      of `CLAUDE_SESSION_ID`/`ANTIGRAVITY_SESSION_ID`/Codex equivalent
      don't exist as named in the source spec.

## Notes

This is infrastructure both [[117]] and [[118]] depend on — land it (or
at least its interface) before or alongside those, not after, since
retrofitting resolution into already-shipped commands means touching
both call sites twice.
