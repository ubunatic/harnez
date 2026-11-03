# 1117 — harnez agent delete reports codex fork/exec error but still removes the agent from the list

**Status**: Closed — removed worktree dir; delete falls back to current dir
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

- Update 2026-11-03 ~23:00: taia hit it three times in a row (`agent delete --name taia31-dev`), so
  not a self-update race. Ruled out: a missing working dir (Go reports that as `chdir <dir>: ...`,
  checked). harnez starts codex directly (`startProcess`, no sandbox, inherited PATH/HOME). Leading
  guess: the caller's shell is sandboxed and cannot see `~/.codex/packages/...`, so the symlink
  dangles there. To check from the failing shell: `readlink -e ~/.local/bin/codex; echo $HOME`.

## 3. Plan
1. Reproduce with a fake start func that returns `fs.ErrNotExist`.
2. Keep the list entry on backend failure, or say clearly what was and was not removed.
3. Verify with a unit test and one live delete of a codex agent.

## 4. Result (2026-11-03)
- Root cause: `taia31-dev`'s working dir `/home/uwe/projects/taia-wt31` was a removed worktree.
  With harnez's exec shape (`CommandContext`, `Setpgid`, `StdoutPipe`, `Start`), Go reports a
  missing `c.Dir` as `fork/exec /home/uwe/.local/bin/codex: no such file or directory`
  (reproduced in a scratch program). The binary was never missing.
- Fix: `CodexDriver.Delete` runs `codex delete` from the current dir when the session dir is
  gone; `startProcess` names a missing dir instead of the misleading exec error. Tests:
  `internal/subagent/codex_dir_test.go`.
- The "disappears from list" part is by design: a failed delete marks the session
  `delete-failed`, hidden from the list and retried by `harnez agent delete --retry-failed`.
- Live: a fresh codex agent (`del-probe`, luna:low) deleted cleanly. taia retries its three
  stuck sessions with `--retry-failed`.
