# 1074 — Sync main to GitHub with a Codeberg CI job, without force or pruning

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1071, 1017

---

/goal Every update of Codeberg `main` reaches GitHub `main` by itself, with a plain fast-forward push that touches no other GitHub ref, verified on a real merge, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

GitHub `main` falls behind Codeberg unless someone runs `git push github origin/main:main`, and a Jules session starts from GitHub. A Codeberg push mirror is not the answer: on 2026-11-01 one filtered to `main` coincided with all 21 other GitHub branches being deleted and 11 PRs closing, so it was removed.

## Want

- A job that runs on a push to Codeberg `main` and runs only `git push github <sha>:refs/heads/main`: no `--force`, no `--prune`, no `--mirror`, and no other refspec.
- A rejected push fails the job visibly, because it means the two `main` branches diverged.
- The GitHub token is a CI secret limited to this repository and never appears in the log.

## Notes

The repository has no Codeberg CI today (`.forgejo/` does not exist). Check that Codeberg runners are available to this repository before building on them. `harnez release` also pushes `main` to both remotes, so check that it tolerates the job having pushed first.
