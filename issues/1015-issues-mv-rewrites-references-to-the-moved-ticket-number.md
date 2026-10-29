# 1015 — `issues mv` rewrites references to the moved ticket number

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1016, 1017, 1013, 1014

---

/goal After `harnez issues mv <old> <new>`, no ticket owned by the current branch still refers to
the old number, and references elsewhere are listed, locked by tests. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a
loop, a hard blocker or no progress, clear the goal and write a failure report here.

## First real use (2026-10-29)

After syncing the fork, upstream main had its own 1012, so the fork branch's 1012 clashed.
`harnez issues mv -d . issues/1012-issues-cli-quick-fixes-….md` worked well: it picked 1014,
renamed the file, rewrote the H1, regenerated README and CLOSED, and committed. Taking the file
path (not the ambiguous number) was the right handle.

But ticket 1013 still said `**Related**: 1012` and "1012 added `--number`". After the move both
silently pointed to upstream's unrelated 1012 (agent selftest). I fixed them by hand.

## Want

- Rewrite `<old>` to `<new>` in `**Related**` lines and plain mentions (word-bounded) in tickets
  that this branch added (`git diff --name-only <merge-base>..HEAD -- issues/`), in the same commit.
- Only *list* mentions in other tickets, since there the old number may now mean the other ticket.
- Print what was rewritten. Commit messages and pushed history stay as they are; say so.
