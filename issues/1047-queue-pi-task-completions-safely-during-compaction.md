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

## Bounded delivery queue: evidence and remaining gaps

`CompletionDelivery` (`.pi/lib/background-task-runner.ts`) holds notifications from this extension's manual `session_before_compact` until `session_compact`/`session_compact_failed`, then submits each at most once, in arrival order, after Pi is idle. Timing is in the `delivery` block of `spec/pi-background-tasks.yaml`.

Same canary before and after the change (real Pi 1.4.0, loopback-only network namespace, real extension and real tasks; `pre.log`, `post.log`):

| Scenario | Before | After |
|---|---|---|
| Three tasks during manual compaction, success | 3 rejected, 0 in transcript | 0 rejected, model receives #2, #1, #3 in finish order, once each |
| Same, summary failure | 3 rejected, 0 in transcript | 0 rejected, same order, once each |
| No compaction: idle and streaming completions | Delivered | Delivered, no hold notes |
| Shutdown during hold | Rejected | Held, then the canary's kill ended the run; the stderr report path is unit-tested only |
| Earlier extension's `session_before_compact` handler holds | 3 rejected | **Still 3 rejected** |
| Follow-up queued before compaction | Stranded silently | Still stranded until the next prompt, now with a warning |

Remaining gaps, so the no-loss goal is not fully met:

1. A completion arriving before this extension's hook runs (another extension's earlier handler is holding) is still rejected and lost as a notification. Pi 1.4.0 exposes no compaction-state API to extensions; the task result stays readable through `background_task_result` until retention evicts it.
2. Pi gives no acknowledgement. A held message whose submission fails after the guard clears (for example missing auth) surfaces only as Pi's extension error; delivery is at most once.
3. Notifications still held at shutdown are not submitted; their IDs go to stderr.
4. The input-handler race and the follow-up stranded by manual compaction are Pi behaviour, unchanged here.
5. Not exercised: the interactive TUI, and a real-extension completion during automatic compaction (the fixture-level canary delivered it once).

Implement and verify a bounded improvement for the reproduced race without claiming the full no-loss goal unless the API limitation is resolved. Preserve ordinary streaming follow-ups and cancellation; make undelivered or unconfirmed work visible, avoid blind resend/duplicate loops, and report residual cases. Keep batching policy in 1044 and output formatting in 1021/1046. Review design and canary evidence before closing; park any genuinely unresolvable gap explicitly.

Before implementing, check live code, Pi APIs and recent history. Coordinate with the output and batching policies in 1021, 1044 and 1046. Verify a real Pi session with a task completing during compaction, multiple concurrent completions, and error/retry handling, plus focused automated tests. Preserve cancellation and normal completion behavior; document any runtime limitation that prevents reliable delivery.
