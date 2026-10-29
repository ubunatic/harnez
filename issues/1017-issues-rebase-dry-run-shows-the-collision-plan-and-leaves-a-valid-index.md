# 1017 — `issues rebase --dry-run` shows the collision plan and leaves a valid index

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1015, 1016

---

/goal `issues rebase --dry-run` prints which tickets collide and what they become, and a real
`issues rebase` ends with all three indexes valid and no manual fix commit, locked by tests. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a
loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Evidence (2026-10-29)

- `harnez issues rebase --dry-run origin/main` on a branch with a real collision (local 1012 vs
  upstream 1012) printed only `next: git rebase origin/main, then harnez repairs and commits the
  listed paths`. No paths were listed, and no collision or target number was shown.
- Upstream: after a rebase, commit 53658d20 is titled "file 1010" but adds `issues/1012-…`, and a
  separate commit c0eb8b6f "reconcile tracker index after rebase" had to fix README by hand
  (10 lines). Assumed cause: the rebase repair renumbered the file but did not regenerate the index;
  verify before fixing.

## Want

- Dry run: list each collision (`ours` file, `theirs` file, new number) and the references that
  1015 would rewrite.
- Real run: regenerate README, BACKLOG and CLOSED in the repair commit; run lint at the end and fail
  loudly when it is not clean.
- Warn when a replayed commit subject names a number that was moved (it cannot be rewritten
  safely; just say so).
