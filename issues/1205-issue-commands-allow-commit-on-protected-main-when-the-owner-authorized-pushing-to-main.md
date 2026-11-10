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
