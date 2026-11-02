# 1102 — pre-push ticket-number lint checks the working tree, not the pushed commits

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1099

---

/goal The pre-push hook checks the ticket files of the commits being pushed against origin/main, not the working tree. Stop and report when blocked on a user decision; on a loop or no progress, clear the goal and write a failure report here.

## Problem

`scripts/git-hooks/pre-push` (added in 1099) runs `harnez issues lint --against origin/main`. `CheckAgainst` (`internal/issues/against.go`) scans `issues/` in the working tree. That causes two problems:

- An uncommitted or untracked ticket file can block a push that does not contain it.
- `git push origin other-branch` is checked against the checked-out branch's files, not `other-branch`.

In the usual flow both sides match: you push the checked-out branch with a clean tree. So the problem is rare.

## Proposed fix

- Add a ref or commit mode, for example `issues lint --against origin/main --rev <sha>`, that lists the tickets with `git ls-tree <sha>`.
- The hook reads the `<local sha>` of each pushed ref from stdin, and skips deletes (the all-zero sha).
- Tests:
  - an untracked colliding file does not block the push;
  - a pushed branch other than HEAD is the one that is checked;
  - the hook test in `internal/githooks`.
