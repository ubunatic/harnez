# 1205 — Issue commands: allow --commit on protected main when the owner authorized pushing to main

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1039 (git.mode + protected_main guard), 1070 (release exception on protected main), 1192

---

## 1. Problem & Motivation

Seen 2026-11-10: the owner authorized pushing to `main` for the session, yet
`harnez issues open --commit` refused with "git.protected_main is set in .harnez/repo.yaml:
refusing to open ticket 1204 on 'main'". The agent had to create `issue/1204-…`, commit there,
push the commit to `main` and delete the branch. Meanwhile a plain `git commit` on `main` in the
same session went through, so the guard only adds detours for ticket commits.

## 2. Goal

/goal `harnez issues <verb> --commit` keeps respecting `git.protected_main` by default, but
commits on `main` when the owner explicitly authorized pushing to main (for example a new
`--push-main` flag, or the existing `/disable repo-rules push-main` session state); covered by
tests in `cmd/harnez/issues_gitflow_test.go`. Stop and ask the owner when choosing between the
flag and the `/disable` state needs their judgment. On a loop or no progress, clear the goal and
write a failure report here.

## 3. Acceptance

- Default unchanged: on protected main without authorization, `--commit` still refuses.
- With authorization, `--commit` commits on `main`; a test covers both cases.
- The refusal message names the override.
- The issue skill's Publish Promptly section and docs mention the override.

## 4. Notes / Unverified

- `internal/disable` already knows the item `repo-rules push-main`; whether the gitflow guard reads
  it is unverified (a quick grep found no link). Reusing it avoids a second consent mechanism, but
  `/disable` is user-invoke only, so an agent-passed flag must not stand in for that consent.
- Check the live guard and recent commits before starting.

## Decision

- 2026-11-10 host (decide mode, smart advisor a1205 agreed): option A. `GuardCommit` honours the
  user-only `/disable repo-rules push-main` state when `disable.Spec.Check` matches the live
  session ID. No `--push-main` flag: any agent could pass it, so it can't carry owner consent.
- Scope: harnez ticket/issue commit paths only. Plain `git commit`, the git hooks and `harnez
  release` (its own exemption, `GuardRelease`) stay unchanged. No force-push.

- 2026-11-10 owner, superseding "No `--push-main` flag" above: A+B. Either unlocks a ticket
  commit on protected main: a `--push-main` flag on the issue commit commands, or the
  `/disable repo-rules push-main` state for the live session. Reason (owner): a non-force push to
  main is always safe; force-push stays forbidden; a push or commit to main is never a release.
  Each override prints one stderr line naming which unlock was used.

## Searcher findings (s1205, codex:luna:low)

- Guard: `internal/gitflow/gitflow.go:237` `GuardCommit`, refusal at 261-266. Callers:
  `internal/issues/status_action.go:146-150`, `cmd/harnez/issues.go:888-891` (mv) and 1053-1057
  (gitAddAndCommit), `internal/issues/maintenance/rebase.go:244`, `reconcile.go:112`.
- Release: `GuardRelease` gitflow.go:219, tests `internal/gitflow/gitflow_test.go:209-242`.
- Disable: `spec/disable.yaml:4-22` (state dir, session env priority, item push-main);
  `internal/disable/disable.go:186-203` `Spec.Check(root, class, item, getenv)`. No production
  code reads push-main today. Host check: `CLAUDE_CODE_SESSION_ID` is set in agent Bash.
- Tests: `cmd/harnez/issues_gitflow_test.go:35-130`. Docs: `docs/commands/disable.md:17` (says
  commits on default stay refused; must change), `docs/commands/issue.md:52-60`, `docs/lang/Git.md:25-29`
  (source; root copy `docs/Git.md`, sync via make install + harnez init -d .).

## M1 (guard honours push-main) edge cases to test

1. No session ID in env → refused as today.
2. State for another session ID (stale) → refused.
3. `/enable` after `/disable` (state removed) → refused.
4. Matching session → commit succeeds; one stderr line names the rule and session.
5. State in another repo root → no effect here.
6. Release tests unchanged; refusal message names the `/disable repo-rules push-main` override.
7. `--push-main` flag alone → commit succeeds (stderr line says "flag"); without flag or state
   → refused, and the message names both unlocks. The flag never touches the release path.
8. Whether disable state has an expiry, and whether an agent can write it, is unverified:
   report what `internal/disable` does; do not add expiry unless it already exists.

## M1 delivered: guard options, --push-main, /disable unlock (772952e2)

Edge case 8: disable state has no expiry; it is a file under `.harnez/state/disable/`, so any process
with write access could create it. Only the owner's `/disable` invocation writes it by convention.

## M2 Pre-Work (reviewer r1205, codex:terra:low)

1. P2: the unlock test counts the phrase "ticket commit unlocked"; assert stderr has exactly one line
   containing it and that the line equals the expected text.
2. P2: add CLI-level tests (through the cobra command, not the handler) proving `--push-main` reaches
   the guard for `issues close`, `issues mv` and one of `rebase`/`reconcile`.
3. P3, host decision: `--push-main` with `--check`, `--dry-run` or `--no-commit` stays accepted as a
   harmless no-op; no change. Reason: rejecting adds flag-conflict code for no safety gain.
