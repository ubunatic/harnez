# 442 — harnez issues open: no commit for newly created ticket

**Status**: Closed — open --commit now commits ticket and index when scoped files differ
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: CLI / Issue tracker
**Related**: 232 (`harnez issues [verb]`), `docs/IssueTracking.md`, `/issue` skill

---

## 1. Problem

The `/issue` workflow ends with `harnez issues open -d <repo> <n> --commit "<msg>"`. For a ticket just created by `harnez issues new`, the placeholder is already `Status: Open`, so the command prints `<n>: already Open (no change)` and returns without syncing the index or committing. The filled-in ticket file and `issues/README.md` stay uncommitted.

Seen on tickets 440 and 441 (2026-09-20): after `--commit` the only new commit was unrelated work; both tickets and the index were committed by hand with `harnez index` and `git commit -- <paths>`.

## 2. /goal

`harnez issues <verb> <n> --commit "<msg>"` commits the ticket file and the regenerated index even when the status does not change, so the documented filing workflow ends with a commit. Only the ticket and `issues/README.md` are staged (with `git commit -- <paths>`), never unrelated working-tree changes.

## 3. Notes

- Reproduce first: a test that creates a ticket with `new`, edits the body, runs `open --commit`, and asserts a commit exists containing the ticket and index.
- Decide whether "no change" should still index and commit when the file or index differs from HEAD. Recommended: yes when `--commit` is given and there is something to commit; a clean tree stays a no-op.
- Re-verify against live `cmd/harnez/issues.go` first.
