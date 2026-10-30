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

## Verified reproduction and delivery boundary

A deterministic local-stub canary in real Pi 1.4.0 reproduced lost completion messages during manual compaction, including multiple tasks; `session_compact` fires before the guard clears while `session_compact_failed` fires afterward. The runtime `sendUserMessage` wrapper returns no usable acknowledgement, so general exactly-once delivery across all races cannot be guaranteed by this extension alone. An input-handler race and pre-existing queued follow-up stalling were also observed. The canary is temporarily at `/tmp/pi-1047-canary.ck5aSM/`; it uses a local stub, not an external model.

Implement and verify a bounded improvement for the reproduced race without claiming the full no-loss goal unless the API limitation is resolved. Preserve ordinary streaming follow-ups and cancellation; make undelivered or unconfirmed work visible, avoid blind resend/duplicate loops, and report residual cases. Keep batching policy in 1044 and output formatting in 1021/1046. Review design and canary evidence before closing; park any genuinely unresolvable gap explicitly.

Before implementing, check live code, Pi APIs and recent history. Coordinate with the output and batching policies in 1021, 1044 and 1046. Verify a real Pi session with a task completing during compaction, multiple concurrent completions, and error/retry handling, plus focused automated tests. Preserve cancellation and normal completion behavior; document any runtime limitation that prevents reliable delivery.
