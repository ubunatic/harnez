# 1011 — Show labeled Pi background tasks in the status line

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1009, 1003

---

## 1. Problem & Motivation

The Pi background-task status currently shows only `Tasks: <num>`, so the user cannot tell which tasks are running at a glance.

## 2. Goal

/goal Show the running task count and compact labeled entries in the Pi status line, verify the behavior, and stop and report if blocked by a user decision or denied permission. On a loop, hard architectural or OS blocker, or lack of progress, clear the goal and record a failure report.

Use the agreed format, for example:

```text
Tasks (3): ¹🔨build, ²🧪tests, ³👤backup
```

The calling agent supplies a single-word label when creating a task; manual `/task` jobs use the fixed 👤 emoji. Bound the display for narrow terminals or many tasks while retaining the total, for example `Tasks (7): ¹🔨build, ²🧪tests, ³👤backup, +4`. Preserve task execution, completion notifications, and existing workflows. Recheck current code and recent history before implementation; determine safe defaults and compatibility for existing callers.

## 3. Verification

Test agent-created and manual tasks, count and label updates as tasks finish or are cancelled, and bounded rendering including multi-digit IDs and emoji display width. Verify the status in a live Pi session as well as automated tests; report any environment limitations.
