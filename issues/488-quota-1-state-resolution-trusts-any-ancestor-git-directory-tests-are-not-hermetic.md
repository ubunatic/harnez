# 488 — Quota-1 state resolution trusts any ancestor .git directory; tests are not hermetic

**Status**: Closed — a .git dir counts only with HEAD; junk-ancestor and worktree tests (a7d34a6)

**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: #479, `docs/feedback/2026-09-22-orchestrated-sprint-flow-report.md`, `internal/quota1`

---

## 1. Problem & Motivation

`quota1.ResolveStateFile` walks up from the working directory and accepts the first
`.git` *directory* it finds, without checking that it is a real repository, and
`writeState` then creates `<that dir>/harnez/quota_1.state`.

During the orchestrated sprint a worker ran a Quota-1 command with a working directory
under `/tmp`. That left `/tmp/.git/` holding only `harnez/quota_1.state` and an empty
`info/`. Afterwards `TestCheckAndRecord_Bypass` and `TestCheckAndRecord_NonGitDir` failed
for everyone on the machine: the "non-git" test directory (also under `/tmp`) resolved into
the junk `/tmp/.git`, so its state file was `/tmp/.git/harnez/quota_1.state`. Deleting the
junk directory fixed it. Workers had earlier reported unexplained "exec-hook assertion
failures" that passed everywhere else; this class of shared-`/tmp` state is a likely cause.

## 2. Specification

- Treat a `.git` directory as a repository root only if it looks like one (contains
  `HEAD`), otherwise keep walking or fall back to `.harnez` in the start directory.
- Never create a `.git` path component: state lives in an existing repository's `.git`
  or in `<dir>/.harnez`.
- Tests build their own root with `GIT_CEILING_DIRECTORIES` or an explicit fake root so
  a stray directory in `/tmp` cannot change results.

## 3. Verification

- Tests: a junk `.git` directory without `HEAD` above the start directory is ignored; a
  real repository is still found; nothing creates a `.git` directory; both existing tests
  pass with a junk `/tmp/.git` present.

## Pre-Work (lean sprint, 2026-09-22)

- Keep the existing worktree/submodule branch (`.git` is a *file*, `quota.go:55`): it must still
  resolve to `.harnez` in the worktree root. Add a test for it next to the new junk-`.git` case.
- Tests must not depend on the real `/tmp`: build the junk `.git` inside `t.TempDir()` above the
  start dir, and set `GIT_CEILING_DIRECTORIES` where a test relies on "not a repo".
