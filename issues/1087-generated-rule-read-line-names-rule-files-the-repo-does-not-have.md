# 1087 — generated rule-read line names rule files the repo does not have

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug

---

/goal The first line that `harnez init` writes into `AGENTS.md` and `CLAUDE.md` names only rule files that exist in that repo, or tolerates missing ones explicitly, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

Reported by the sitegen session on 2026-11-01 and confirmed there and in termaid and books. The line tells agents to run `harnez read` on `.harnez/rules/Quota.md` and `Local.md`, but those repos lack both files. sitegen's `.harnez/repo.yaml` sets `quota: false`, and `Local.md` is per-checkout and git-excluded, so a fresh clone lacks it too.

The line's text is in `config.yaml`, so it looks fixed rather than derived from the repo's settings. It now sits in the repos refreshed by the 2026-11-01 sync (1084).

## Want

Build the file list from the rules init actually renders, or have `harnez read` and the line state that absent optional files are skipped. Check what `harnez read` does today with a missing path. Pin the result with a test in `internal/claude/rules_init_test.go`.
