# 1053 — Git.md: add a breaking-change commit example

**Status**: Closed — Already done: the example is present at docs/lang/Git.md:50-55. Found by the Jules audit, reports/jules/1053.md (1075).
**Priority**: P1 (High)
**Severity**: Minor
**Category**: Docs
**Related**:
**Forgewerk**: run

---

/goal `docs/Git.md` shows how to mark a breaking change in a commit message. Stop and report if
anything is unclear.

## Task

In `docs/Git.md`, section "Commit messages", add one bullet with a breaking-change example:
`feat(cli)!: drop the --old flag`, plus a `BREAKING CHANGE:` footer, as in Conventional Commits.
Change nothing else.

This is a live test of forgewerk (flashcoder/forgewerk ticket 022): one server, two repos.
