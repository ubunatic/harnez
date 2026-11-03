# 1117 — harnez agent delete reports codex fork/exec error but still removes the agent from the list

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**:

---

/goal Make `harnez agent delete` on a codex agent either delete the codex session or fail without
dropping the agent from `harnez agent list`, with a test for the missing-binary case; or stop and
report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Reported by the taia session (2026-11-03): `harnez agent delete` failed with
`codex delete: fork/exec /home/uwe/.local/bin/codex: no such file or directory`, yet the agent
then disappeared from `harnez agent list`. So harnez forgets the session while the codex
rollout may stay behind, and the error says nothing about the half-done state.

## 2. Findings (unverified)
- The error comes from `internal/subagent/codex.go` (`codex delete --force <id>`).
- At filing time `~/.local/bin/codex` is a symlink to `~/.codex/packages/standalone/current/bin/codex`
  and resolves (codex 0.166.0, symlink dated 2026-11-01). Likely cause: the codex self-updater swapped
  the symlink while delete ran, or taia ran with a different PATH. Not reproduced.
- Open: does the list entry get removed before or after the codex call, and should a failed
  backend delete keep the entry (or report "removed from list, codex session kept")?

## 3. Plan
1. Reproduce with a fake start func that returns `fs.ErrNotExist`.
2. Keep the list entry on backend failure, or say clearly what was and was not removed.
3. Verify with a unit test and one live delete of a codex agent.
