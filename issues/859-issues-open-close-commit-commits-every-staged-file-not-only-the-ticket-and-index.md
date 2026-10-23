# 859 — issues open/close --commit commits every staged file, not only the ticket and index

**Status**: Closed — explicit-path commits preserve unrelated staged work across open, close, and mv tests; absent indexes skipped
**Priority**: P1 (High) — silently mixes another agent's work into a tracker commit
**Severity**: Major
**Category**: Bug
**Related**: 232 (`harnez issues` verbs with commit-by-default)

---

/goal `harnez issues <verb> --commit` (and the default commit of close) commits only the ticket file and the
regenerated index, leaving other staged changes staged and uncommitted; reproduction test first; or stop
and report when blocked on a user decision.

## Reproduction (loom, 2026-10-22)

A developer agent had staged its work (script, tests, docs, Makefile) for its own commit. The host ran
`harnez issues open -d . 365 --commit "docs(issues): file 365, ..."`. Commit 46905a3 contained all seven
staged files under the ticket's message and without the developer's trailer; the developer's own commit then
had nothing to commit. Host and developer share one working tree in lean sprints, so this happens whenever a
ticket is filed while a developer is between `git add` and `git commit`.

## Fix direction

Commit with explicit paths (`git commit -m ... -- <ticket> <index>`), which ignores the rest of the index.

## Sprint findings (2026-10-24, searcher argon:low)

- `gitAddAndCommit` (`cmd/harnez/issues.go:864-884`) runs `git commit -m <msg>` without pathspec (`:871`).
  Callers already pass exact paths: status verbs (`internal/issues/status_action.go:97-100, 167-170, 292-300`),
  `issues mv` (`cmd/harnez/issues.go:767-775`, incl. cross-repo reference edits).
- Caveat: `issues/BACKLOG.md` / `issues/CLOSED.md` may not exist; `git commit -- <path>` fails on an unknown pathspec.
- Tests: `cmd/harnez/issues_test.go`, `issues_close_test.go`, `issues_duplicates_test.go`.
