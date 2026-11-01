# 1017 — `issues rebase --dry-run` shows the collision plan and leaves a valid index

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
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

- Another real collision (local self-test issue 1010 vs upstream 1010) stopped replay at the generated-index merge driver with `conflicting generated rows for 1010`, before automatic repair. Resolving it required manually renumbering the local ticket during the stopped rebase and regenerating the index afterward. Verify this failure path rather than assuming replay always reaches the repair step.

## Want

- Dry run: list each collision (`ours` file, `theirs` file, new number) and the references that
  1015 would rewrite.
- Real run: regenerate README, BACKLOG and CLOSED in the repair commit; run lint at the end and fail
  loudly when it is not clean.
- Warn when a replayed commit subject names a number that was moved (it cannot be rewritten
  safely; just say so).

## Attempt (2026-11-01)

Jules opened [GitHub PR 37](https://github.com/ubunatic/harnez/pull/37) as the real test of 1071. Not reviewed
or merged. It changes `rebase.go` and `reconcile.go`, adds no Go tests (the goal wants tests) and adds two
throwaway scripts at the repository root (`test-dry-run.sh`, `test-real-run.sh`). Its base is GitHub `main`.
PR 37 was closed and its branch deleted on 2026-11-01 when a push mirror removed the GitHub branches; the owner
chose not to restore them. Its commit `591930b0` is still readable as GitHub's `refs/pull/37/head`. Correction (2026-11-01): the branch `fix-issues-rebase-6379860427770255774` does exist on GitHub, at a later commit `ee3963c4` of the same Jules session.

## Root cause (2026-11-01)

`collectLocalTickets` (`internal/issues/maintenance/rebase.go:139`) only recognises three-digit ticket numbers: it checks `base[3] != '-'` and takes `base[:3]`. A ticket numbered 1000 or above is never "owned", so its collision is never repaired, and the rebase stops with `issues lint: duplicate ticket number`. This tracker passed 1000, so repair no longer works for any new ticket. It also replaces the assumed cause in the Evidence section.

Found from three independent Jules drafts of a failing test (1075). Running draft `pr59` in a scratch clone at `c6832711` fails with number 1010 and passes with number 270, using the same setup. The existing test passes only because it uses 270 and 271.

Start from `reports/jules/drafts/rebase_1017_pr59.go.txt`, which holds separate tests for the dry run and the real run. All three drafts fail on the missing dry-run output for the right reason.

Two smaller defects in the same file:

- `rebaseStatePath` (`rebase.go:54-61`) asks git for the `harnez/issues-rebase.json` path and then joins `harnez/issues-rebase.json` onto it again, so the state file lives at `.git/harnez/issues-rebase.json/harnez/issues-rebase.json`.
- `trackerDirty` (`rebase.go:120`) keeps only the destination of a rename, so `git mv issues/001-a.md docs/001-a.md` does not count as a dirty tracker. See `reports/jules/code-audit-issues-maintenance.md`.
