---
kind: sprint
date: 2026-11-10
ticket: 1208
---

# Lean sprint 1208: agent start/resume --dry-run

## Flow

- Searcher s1208 (codex:luna:low): stop points in `cmd/harnez/agent_run.go` (start before the Codex
  preflight, resume before the stale repair), argv builders per driver, fake-CLI test helpers.
- Host decision: one milestone; extract a pure agy argv builder; print the prompt as a byte count.
- M1 (dry-run flag, 5a440c8b): host full run 206 ok after a golden fix; live dry runs for agy, claude
  and codex were correct.
- Reviewer r1208 (codex:terra:low): the prompt was echoed in a flag-like-prompt error, cobra-level
  side-effect tests were missing, and the agy empty-conversation argv had changed.
- M2 (hardening, 23d51793): round 1 failed the host full run because a dry run created
  `telemetry.sqlite` (quota lookup opened the store read-write). The fix added a read-only quota lookup.
  The host full run then gave 206 ok, 0 FAIL.

## Verification

- `harnez init -d .` left `git status` clean; the `~/.harnez/agents` count was unchanged across a live dry run.
- `-p "--secret-token ..."` now reports only the flag token and byte count.

## Learnings

- The new cobra-level side-effect test caught a real leak (telemetry DB creation) that the unit tests missed.
  End-to-end "nothing changed on disk" tests are worth their cost for any preview flag.
