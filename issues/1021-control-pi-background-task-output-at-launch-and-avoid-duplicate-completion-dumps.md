# 1021 — Control Pi background task output at launch and avoid duplicate completion dumps

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1003, 1009, 1011, 1044, 1046; `.pi/extensions/background-tasks.ts`; `.pi/lib/background-task-runner.ts`

## Refinement: owner decisions needed

Read-only discovery confirms the runner retains a 16,000-character tail for up to 50 finished tasks; generic shell output cannot reliably identify an agent final message. Owner decisions: file-backed capture is the default and may be opted out of per task, including consideration of manual tasks. Use private files on a best-effort basis; a short completion tail is acceptable. Delete captures when the session closes (and on earlier eviction); best-effort crash cleanup must not delete other live sessions' files. On disk-full or write failure, keep the task running, explicitly mark capture incomplete and retain the readable portion. Do not impose an arbitrary capture cap or claim incomplete output is complete. Provide explicit paged retrieval and document the privacy and crash limitations. 1044 batching is sequenced after this policy.

## Architect refinement and decisions

Opus:med proposed a serialized asynchronous capture writer with bounded in-memory queue and stream backpressure, explicit incomplete states for stall, write failure and drain timeout, and lifecycle gates for finalization, eviction and shutdown. Use private temporary storage respecting `TMPDIR` (not a new persistent cache); it may be memory-backed and exhaust space, so errors must be visible. Preserve captured prefixes and never claim full capture after loss. Use spec/schema for configurable watermarks and deadlines, not a transcript size cap. In-flight filesystem operations cannot necessarily be cancelled; report residual cleanup and crash limitations. Test delayed and partial writes, timeouts, late callbacks, cancellation, cleanup and live Pi behavior. S0 `ae2e2886` temporarily disables the unsafe synchronous capture path for ordinary production starts; the default-on owner policy remains pending. Developer `make test-q1` passed and host inspected the diff. S1 unwired writer is in progress: its first focused test stopped before execution because Node strip-only mode rejects TypeScript parameter properties. Replace those with explicit fields, rerun only the touched focused test after the edit, and report the full suite owed; preserve all assertions. Next review the writer before integration. The existing M1 remains unaccepted until these review findings are fixed. Defer a crash sweep unless ownership and liveness can be established safely. 1044 handles batching and 1046 presentation; neither changes Pi's lack of delivery acknowledgement.

## Slice B MVP

File capture defaults on; tool calls can set `capture: false` and `/task --no-capture` opts out. Completion reports capture status, committed bytes and path rather than repeating the transcript; results retain the bounded tail. Files are private (0700 directory, 0600 file), but not isolated from the same user. TMPDIR can be tmpfs and consume RAM; crashes or hung I/O can leave residual files. Session close and eviction perform best-effort cleanup. A real Pi canary must verify tool schema and opt-out, paged `read` access, completion delivery, failure reporting and cleanup in a live session; the full Quota-1 suite remains owed.

## MVP review pre-work

Sol review of `71882284` and `17828892`: held compaction notifications may outlive the task's retention slot and capture file, producing dead paths. Protect tasks with held notifications from eviction until submitted, and regression-test more than the retention limit during compaction; respect 1047's no-ack limitation. Completion should also surface spawn errors rather than advertising an empty complete capture without the diagnostic. Document that the bounded post-exit drain can mark even finite buffered output incomplete when storage is slow.

## Runner integration review pre-work

Sol review of `695f7c8d` found an MVP blocker: after the leader exits, backpressure clears/suppresses the drain timer, while steady slow writes reset the writer stall timer. An escaped descendant can therefore keep completion and eviction pending indefinitely. Bound post-exit draining across pause/resume cycles, mark discarded output incomplete, and test continuing progress without stream closure. Bound the new tests' deletion polling as well. Store late-open close failure remains a documented nonblocking cleanup limitation.

## Payload groundwork review pre-work

`2952cbf2` adds two installer payloads and capture configuration. Host Quota-1 failed `TestPiBackgroundRevertDeactivatesAndApplyRecovers`: apply after revert changed six paths, but the assertion still expects four (`pi_background_test.go:661`). Update the assertion to verify all six exact intended paths, preserving recovery assertions, and run focused tests. The full suite is owed after correction.

## S2 store and integration

Standalone store `e6c3c9d5` is unwired; developer reports Quota-1 passed with 122 Pi tests. Sol found no standalone MVP blocker. During integration coordinate writer abort/detach before store shutdown so pending output is marked incomplete and streams resume; creation limiter does not limit writes or cleanup. Known nonblocking hardening: a late-open lease whose close rejects skips removal and reports a residual path; fix if straightforward during integration or track for follow-up. Same-user identity checks are best-effort, and hung I/O may outlive shutdown.

## S1 writer review pre-work

Standalone writer `f0e60868` is unwired; host Quota-1 passed 109 Pi tests. Fix `87795895` addresses most findings; a second sol review found rejected or invalid writes are incorrectly counted as confirmed dropped even though a prefix may already have reached storage. `7d52f5a8` classifies them as uncertain and adds a prefix-written-then-rejected regression; host Quota-1 passed and sol's one-shot review found no blocker to building the store. Production integration remains unreviewed. Developer reports Quota-1 passed 115 Pi tests after the preceding fixes. Sol review requires changes before integration: copy Buffer input rather than sharing `slice()` storage; ensure detaching while paused cannot strand streams; expose a separate cleanup/residual-operation gate and late I/O errors; handle `Promise.reject(undefined)` as close failure; make truncation flush accepted bytes and never allow a false complete result. Distinguish uncertain in-flight bytes from confirmed dropped bytes. Add regressions for buffer reuse, paused detach, callback throws, synchronous I/O throws, invalid write counts, late errors, cleanup and independent streams. Preserve assertions and keep writer unwired until reviewed.

## M2 pre-work: M1 capture review

M1 `62b1356f` adds private captures; developer reports Quota-1 passed. Terra stopped before implementing review fixes: pending asynchronous writes race drain timeout and shutdown/cleanup, and a safe verified lifecycle design is needed. M1 is not accepted for release. Sol review requires fixes before extending the API: replace synchronous event-loop-blocking writes with asynchronous bounded backpressure; visibly report capture creation/write/close failures while tasks continue; mark drain-timeout-discarded output incomplete; unwind descriptor and file if creation fails after opening; and clean up the direct-runner test's capture. Cover creation/close errors, slow storage, cancellation/drain and path replacement. Inode checks are best-effort, not atomic protection against same-user races. Do not weaken prior assertions. Preserve owner-approved session cleanup and no arbitrary cap; crash or kill may leave private orphan files and must be disclosed.

## Problem

During sitegen epic 008, the agent inspected completed background jobs while orchestrating work, then delayed completion messages displayed their full outputs again as separate user-visible messages. Fourteen large repeated notifications interrupted the conversation and were also presented to the agent. Shell redirection suggested as a workaround relies on each agent remembering to pipe output, which the owner does not want to rely on.

## Goal

/goal Give the caller an explicit, launch-time output policy for Pi background tasks, automatically capture and deliver output according to that policy without shell redirection, and verify useful failure diagnostics and on-demand retrieval without duplicate full-output notifications; or stop and report when blocked on user input or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Before implementation, re-check live plugin code, installed/global extension coordination, current behavior and recent history. The agent should select at task creation whether it wants full output delivered immediately on completion or captured to a temporary file for later inspection; in file mode a successful task should emit only a compact completion/status and retrieval location, while a failed task must make its output available for diagnosis. The owner expects the agent to read output on failure or when it chooses, rather than have every completion dump it automatically. Define bounded failure presentation without losing full output, exit/signal information or the ability to inspect results. Avoid repeating output already inspected; preserve reliable lifecycle notifications and existing cancellation semantics.

Owner clarification: long task output should go to a file so the host can choose what to read: only the final message, a bounded portion or the full transcript, or nothing when it already knows enough. Avoid loading the transcript into the host context merely to deliver a completion. Reconcile this requirement with the launch-time policy above, and distinguish generic command output from agent transcripts where a final message can be identified reliably.

The plugin, not the shell command or an agent-authored pipe, owns capture, file creation, output lifetime and access. Define safe temporary-file permissions, retention/cleanup, size/truncation behavior, compatibility and defaults for existing callers and manual tasks. Add tests for both policies, success/failure, result inspection before/after completion, large and sensitive output, cleanup and concurrent tasks; verify a real Pi session's user and agent notifications as well as automated tests. Do not claim shell redirection or a fake UI test alone proves the desired behavior.
