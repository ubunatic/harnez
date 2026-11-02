# 1089 — Jules start: refused run keeps its name, vague refusal, unexplained compare failure

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1071, 1074, 1088

---

/goal A refused Jules start frees its session name and says which setup step is missing, and the compare failure below is explained or fixed, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

Reported by the uman session on 2026-11-01 from Jules runs on `ubunatic/uman` at `0506459`. Not yet reproduced here. Jules takes no new work after 2026-11-11, so fix this before then or close it as moot.

- A run refused by a pre-flight check still holds its session name, so a retry fails with `session name ... is already in use`.
- The refusal says `verify the provider/model configuration`, but the `github_projects` entry and the jules.google.com connection are separate checks, so one setup produced two errors in a row.
- One run printed `could not compare github/main with origin: exit status 128` and continued. The cause is unknown.

## Want

A refused start releases the name, and its message names the missing step. Find the cause of the compare failure, and include git's stderr in that warning.
