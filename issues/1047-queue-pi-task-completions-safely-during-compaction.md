# 1047 — Queue Pi task completions safely during compaction

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 1021, 1044, 1046; `.pi/extensions/background-tasks.ts`

## Problem

A background task finished while the host session was compacting, producing:

```text
Extension "<runtime>" error: Cannot submit a prompt while compaction is in progress. Wait for compaction to finish and retry.
```

The completion's delivery and whether it can be lost or duplicated are not yet established. Investigate the actual Pi runtime and extension interaction rather than assuming the error originates in a particular layer.

## Goal

/goal Make task completion delivery robust when Pi is compacting: retain and deliver pending completion information after compaction without loss, duplicate delivery, or a retry loop; verify the behavior, or stop and report when blocked on user input or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Before implementing, check live code, Pi APIs and recent history. Coordinate with the output and batching policies in 1021, 1044 and 1046. Verify a real Pi session with a task completing during compaction, multiple concurrent completions, and error/retry handling, plus focused automated tests. Preserve cancellation and normal completion behavior; document any runtime limitation that prevents reliable delivery.
