# 1016 — `issues reconcile` repairs number collisions after a `git merge`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1015, 1017, 1013

---

/goal One command, run after `git merge <upstream>` on a branch with ticket number collisions,
renumbers the branch's own colliding tickets, rewrites their references (1015), repairs all three
indexes and commits once, without rewriting history, locked by tests and one canary on a real
merge. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a
loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Why

`issues rebase` replays commits, so it cannot be used on a pushed PR branch (forks here merge
upstream into the branch; rebase and force-push are not allowed). On 2026-10-29 the merge of
upstream main into `fix/1012-issues-quick-fixes` needed by hand:

1. `git merge`: the index hook failed with "duplicate ticket number 1012 … renumber local numeric
   extras with harnez issues mv" (good hint), but the merge commit was still made.
2. Find which 1012 was ours.
3. `issues mv <file>`.
4. Fix references in our other ticket by hand.

## Want

- `harnez issues reconcile [<ref>]` (default: the merged parent, else `origin/main`): ownership =
  ticket files added in `<merge-base>..HEAD` but not in `<ref>`; the owned duplicate moves to the
  next free number (respecting 1013 once it lands), via the same code as `mv` and 1015.
- `--dry-run` prints the plan (old → new per file, references to rewrite).
- Optionally run it from the merge hook instead of only printing the hint (opt-in in `spec/`).
- Shares ownership and renumbering code with `issues rebase` (1017), not a second copy.
