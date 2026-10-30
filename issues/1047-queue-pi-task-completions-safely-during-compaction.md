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

A deterministic local-stub canary in real Pi 1.4.0 reproduced lost completion messages during manual compaction, including multiple tasks; `session_compact` fires before the guard clears while `session_compact_failed` fires afterward. The runtime `sendUserMessage` wrapper returns no usable acknowledgement, so general exactly-once delivery across all races cannot be guaranteed by this extension alone. An input-handler race and pre-existing queued follow-up stalling were also observed. The canary is now `scripts/canary-pi-compaction/` (`make canary-pi-compaction`); it uses a local stub, not an external model.

## Delivery queue: evidence and remaining gaps

`CompletionDelivery` (`.pi/lib/background-task-runner.ts`) holds notifications from this extension's manual `session_before_compact` until `session_compact`/`session_compact_failed`, then submits each at most once, in arrival order, after Pi is idle. Timing is in the `delivery` block of `spec/pi-background-tasks.yaml`.

Same canary before and after the change (real Pi 1.4.0, loopback-only network namespace, real extension and real tasks; `pre.log`, `post.log`):

| Scenario | Before | After |
|---|---|---|
| Three tasks during manual compaction, success | 3 rejected, 0 in transcript | 0 rejected, model receives #2, #1, #3 in finish order, once each |
| Same, summary failure | 3 rejected, 0 in transcript | 0 rejected, same order, once each |
| No compaction: idle and streaming completions | Delivered | Delivered, no hold notes |
| Shutdown during hold | Rejected | Held, not submitted; Pi's stderr shows `Background task notifications not submitted before shutdown: #1` |
| Earlier extension's `session_before_compact` handler holds | 3 rejected | 3 rejected in that first change; fixed by the review pre-work below |
| Follow-up queued before compaction | Stranded silently | Still stranded until the next prompt, now with a warning |

Remaining gaps, so the no-loss goal is not fully met:

1. A run started while an earlier extension's hook holds compaction (for example `sendMessage` with `triggerTurn`) makes the run look known while Pi's guard is set, so a completion can still be rejected. This is reasoned from Pi's code, not reproduced. If `agent_settled` were missed, the same stale state would apply. The task result stays readable through `background_task_result` until retention evicts it.
2. Pi gives no acknowledgement. A held message whose submission fails after the guard clears (for example missing auth) surfaces only as Pi's extension error; delivery is at most once.
3. Notifications still held at shutdown are not submitted; their IDs go to stderr.
4. The input-handler race and the follow-up stranded by manual compaction are Pi behaviour, unchanged here.
5. A completion in the short window between Pi starting a run and `agent_start` is held until `agent_start`, and polling continues until Pi is idle, a run is known or the session shuts down.
6. Not exercised with the real extension: interactive TUI.

## Host canary and pending review

Host `make canary-pi-compaction` passed all 14 scenarios after `eac42623`. The shutdown scenario nevertheless emitted an `extension_error` saying the captured extension context was stale, alongside the expected stderr report of held task #1; passing scenario assertions do not establish that shutdown is error-free. Independent review must investigate this before acceptance. Claude reviewer dispatch failed on the provider's five-hour quota; no alternate provider was used.

## Shutdown fix review and pre-work

`62466a45` detaches the captured status callback before task shutdown. Developer reports Quota-1 and 14 real-Pi scenarios passing, including shutdown with held ID on stderr and no subsequent extension error. One-shot sol review accepted the bounded fix but found the canary's `startTasks()` clears earlier recorded events: its zero-error assertion only covers later events. Preserve or independently track extension errors across the entire shutdown scenario, without weakening the intentional rejection scenario, then verify. Full no-loss delivery remains unproven because Pi provides no acknowledgement; keep this issue open.

## Review pre-work

Independent review found no critical/high blocker but identified missing committed tests for extension hook wiring, and the real Pi canary existed only in `/tmp`. Add deterministic wiring tests, preserve a maintainable canary in the repo, and test whether agent lifecycle plus `isIdle` can prevent the earlier-hook loss (scenario 17) without breaking normal streaming. Recheck idle on every drain step, guard compaction-end reason where applicable, ensure stranded-follow-up warning is not lost to a slow later hook, and reduce success warning noise. Do not claim acknowledgement or exactly-once where Pi cannot provide it. Avoid weakening existing assertions or hiding remaining limits.

Done: `CompletionDelivery` treats "not idle with no known agent run" as possible compaction, and rechecks on every drain step. Compaction-end hooks act only on manual reason, the stranded check waits out slow later hooks, and one summary replaces per-task success warnings. The test `the first run is active…` now calls `agentStarted` before follow-ups and asserts one summary instead of three notes, because that is the changed behaviour; no assertion was removed for any other reason.

`scripts/canary-pi-compaction/` runs 14 real-Pi scenarios with the real extension, all passing on Pi 1.4.0: normal idle and streaming, runtime contract (direct send rejected during a hold), three tasks with success, summary failure and extension cancel, earlier-hook hold (scenario 17: 0 rejected, #2, #1, #3 once each), RPC abort, completion during summary, drain follow-ups, compact aborting a run then earlier-hook hold, slow later hook, automatic compaction, stranded warning, shutdown.

Implement and verify a bounded improvement for the reproduced race without claiming the full no-loss goal unless the API limitation is resolved. Preserve ordinary streaming follow-ups and cancellation; make undelivered or unconfirmed work visible, avoid blind resend/duplicate loops, and report residual cases. Keep batching policy in 1044 and output formatting in 1021/1046. Review design and canary evidence before closing; park any genuinely unresolvable gap explicitly.

Before implementing, check live code, Pi APIs and recent history. Coordinate with the output and batching policies in 1021, 1044 and 1046. Verify a real Pi session with a task completing during compaction, multiple concurrent completions, and error/retry handling, plus focused automated tests. Preserve cancellation and normal completion behavior; document any runtime limitation that prevents reliable delivery.
