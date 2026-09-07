# 279 — Avoid persistent issues README lock sidecar in working trees

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[232-harnez-issues-verb-command-for-single-call-status-changes-with-index-sync-and-commit]], [[238-harnez-init-doesn-t-gitignore-issues-readme-md-lock-in-managed-repos]], `internal/index/index.go`

---

## 1. Problem & Motivation

Users often observe `issues/README.md.lock` lingering after `harnez index` and
`harnez issues <verb>` complete. This looks like a stale lock or an interrupted
operation even when no process still holds it. Ticket 238 stopped the sidecar
from appearing as untracked Git noise, but hiding the file does not address the
confusing persistent artifact or explain its lifecycle.

The current implementation deliberately opens the sidecar with `O_CREATE`,
takes an advisory kernel `flock`, then unlocks and closes it without unlinking.
The live lock therefore releases automatically on normal exit, error, crash, or
kill; the empty file's continued existence does not mean the lock is held.

## 2. Findings & Scope

- Preserve a stable inode for coordination between concurrent `harnez index`
  and `harnez issues <verb>` processes. Naively deleting the sidecar on unlock
  is unsafe: a waiter may already have the old inode open while another process
  creates and locks a replacement, allowing two independent critical sections.
- Investigate moving the stable lock target out of the tracked worktree, such as
  an appropriate repository-local Git state directory. Account for worktrees,
  bare/non-Git managed directories, permissions, and a deterministic fallback.
- If relocation is not sufficiently portable, make the persistent-sidecar
  contract explicit in command help/status or diagnostics so an unlocked file
  is not presented or interpreted as stale state.
- Keep the existing non-blocking, bounded-retry contention behavior and ensure
  lock acquisition still covers the complete README read-modify-write cycle.
- Do not add PID/timestamp stale-lock deletion: file contents/existence are not
  the ownership signal for `flock`, and kernel-held locks already recover from
  interrupted processes.
- Consider whether README writes should become temp-file-plus-rename atomic.
  This is separate from lock ownership, but interruption currently can leave a
  partially written index because `os.WriteFile` truncates the destination.

## 3. Acceptance Criteria

- [ ] Successful index and issue-status operations do not leave a misleading
      lock artifact in the user-visible worktree, or the persistent sidecar is
      clearly documented and diagnosed as an unlocked coordination inode.
- [ ] Two concurrent index/issue operations remain serialized on one stable
      lock target; tests cover contention across the chosen location.
- [ ] Tests cover normal completion and process interruption, confirming that
      no stale ownership blocks the next invocation.
- [ ] Any cleanup strategy is tested against the open-before-unlink inode race;
      unlink-on-unlock must not be introduced without solving that race.
- [ ] README update failure/interrupt behavior is evaluated, with atomic
      replacement implemented or tracked separately if partial writes remain
      possible.
- [ ] Existing managed repositories and Git worktrees have a compatible
      migration/fallback path.

## 4. Verification Guidance

- Add focused `internal/index` tests for lock reuse, bounded contention, and
  crash/interruption recovery (using a helper subprocess where needed).
- Exercise concurrent `harnez index` and `harnez issues <verb>` invocations
  against the same repository and verify a valid, complete `issues/README.md`.
- Run `go test ./...`, `make install`, and the relevant CLI smoke tests.
