# 976 — harnez issues help should state per-verb commit and staging rules briefly

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 975 (find issues Priority), 318 (issues list verb)

---

/goal `harnez issues <verb> --help` states, in one line per verb, which files it writes and commits, whether it stages
other working-tree files, and what `--commit` and `--no-commit` do; or stop and report when blocked on a user decision
or denied permission.

## 1. Problem & Motivation

An agent checking `harnez issues open --help` could not tell from the text whether `open` commits only the ticket
and README or every changed file. The help says "commits the changed files by default" in one place and "commits both
files by default" in another, and the answer sits in a long screen that covers every verb. The reader had to guess,
then confirm with `git show --stat` after the commit.

## 2. Technical Specification / Findings

- The status verbs (`open`, `start`, `block`, `close`, `done`, `draft`, `backlog`, `promote`) share one help block
  in `cmd/harnez/issues.go`; confirm the source before editing.
- Wording is inconsistent: "changed files" in the intro, "both files" in the `open` section.
- Missing: whether unrelated modified files are staged, what `--commit "<msg>"` replaces, and what `--no-commit`
  leaves uncommitted.

## 3. Implementation & Verification Plan

1. Give each status verb one line: files written, files committed, and that other working-tree files are never staged.
2. Use one commit-rule wording across the help text.
3. Verify with `harnez issues open --help` on a dirty tree, then `make test-q1` and `make install`.
