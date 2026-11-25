# 1297 — Add direct tests for debug logging

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Infrastructure
**Related**: `internal/logging/logging.go`, `gokay assess`

---

## Problem

`gokay assess` reports `internal/logging` as untested. `Debugf` gates on `DEBUG`, appends timestamped messages to `~/.harnez/debug.log`, and promises best-effort behavior on filesystem errors; the package has no direct tests of those contracts.

## Goal

/goal Add isolated tests for disabled and enabled logging, append behavior, formatting, and an unwritable or missing destination without affecting the caller; verify the relevant tests, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, stop and record the failure and resume point.

Check current code and recent history before implementation; avoid depending on the user's actual home or wall-clock timing.
