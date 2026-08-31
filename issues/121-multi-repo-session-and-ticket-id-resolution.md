# 121 — Multi-repo session & ticket ID resolution strategy

**Status**: Closed — resolved in 32a43c7
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

- [x] Repo-root detection works correctly when invoked from a nested
      subdirectory of a git repo (not just the root). Covered by
      `TestTicket_RepoRootFromNestedSubdir` in
      `internal/resolve/resolve_test.go`.
- [x] Explicit `ticket_id`/`session_id` always overrides implicit
      resolution — covered by a test that supplies both and checks the
      explicit value wins. Covered by `TestTicket_ExplicitOverrideWins`
      and `TestSession_ExplicitOverrideWins`.
- [x] PPID-hash fallback produces a stable session ID across multiple
      `harnez rate`/`harnez exec` calls from the same parent process.
      Covered by `TestSession_PPIDFallbackStableAcrossCalls`.
- [x] Sliding-window lock file: two calls more than 30 minutes apart
      resolve to different session IDs; two calls within the window
      resolve to the same one — covered by a test that manipulates the
      lock file's mtime rather than sleeping 30 minutes. Covered by
      `TestSession_SlidingWindowLockFile`.
- [x] Actual env var names for each agent are confirmed (not assumed)
      before this ticket is marked resolved — note findings here if any
      of `CLAUDE_SESSION_ID`/`ANTIGRAVITY_SESSION_ID`/Codex equivalent
      don't exist as named in the source spec. See "Env var
      verification findings" below.

## Env var verification findings

- **`CLAUDE_CODE_SESSION_ID` — CONFIRMED.** Observed directly via `env`
  in a live Claude Code CLI session while developing this ticket (e.g.
  `CLAUDE_CODE_SESSION_ID=6587aea8-0671-45cf-aad9-783e4664a2ec`). This is
  the real variable name, **not** `CLAUDE_SESSION_ID` as guessed in this
  ticket's original spec — `CLAUDE_SESSION_ID` was not present in that
  environment and is not referenced anywhere else in this repo.
- **`CLAUDE_SESSION_ID` — UNCONFIRMED / not observed.** Kept as a
  secondary candidate (after the confirmed name) in case a future Claude
  Code version introduces it, but it should not be relied on until seen
  in a real environment.
- **`ANTIGRAVITY_SESSION_ID` — UNCONFIRMED.** No Antigravity session was
  available in this repo or its history to inspect, and this repo's
  `docs/studies/*.md` do not document one. A web search turned up no
  documented Antigravity CLI/agent env var for session ID.
- **Codex equivalent — UNCONFIRMED, and does not currently exist.** A web
  search found an open, unimplemented OpenAI Codex CLI feature request
  ("expose current Codex session ID programmatically (env var or JSON)",
  `openai/codex#8923`) confirming Codex has no documented session-id env
  var today. `CODEX_SESSION_ID` is recorded as a placeholder candidate
  only, not a confirmed name.
- **Implementation implication:** `resolve.SessionEnvVars` in
  `internal/resolve/resolve.go` is an ordered slice with the confirmed
  name first, so adding or correcting a name later (once Antigravity or
  Codex actually ship one) is a one-line change. Each entry's
  confirmed/unconfirmed status is documented inline as a comment.

## Notes

This is infrastructure both [[117]] and [[118]] depend on — land it (or
at least its interface) before or alongside those, not after, since
retrofitting resolution into already-shipped commands means touching
both call sites twice.

## Implementation

Landed as `internal/resolve` (package `resolve`):

- `resolve.Session(resolve.SessionOptions) (string, error)` — explicit
  override -> `SessionEnvVars` (ordered, see verification findings above)
  -> PPID-derived sliding-window lock file at
  `~/.harnez/sessions/<hash(ppid)>.lock` (`SessionInactivityWindow` =
  30 min).
- `resolve.Ticket(resolve.TicketOptions) (string, error)` — explicit
  override -> nearest git repo root dir name + branch name (if
  ticket-shaped, i.e. matches `^(prefix/)?NNN-slug$`) -> most-recently-used
  ticket_id recorded for the resolved session at
  `~/.harnez/sessions/<hash(session_id)>.ticket`.
- `resolve.DefaultStateDir()` — `~/.harnez/sessions`.

All options structs accept injectable `Getenv`/`PPID`/`Now`/dir overrides
for deterministic testing (see `internal/resolve/resolve_test.go`).
`harnez rate` ([[117]]) and `harnez exec` ([[118]]) should call these two
functions rather than re-implementing resolution.
