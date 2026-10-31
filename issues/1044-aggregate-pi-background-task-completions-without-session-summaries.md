# 1044 — Aggregate Pi background task completions without session summaries

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 1011, 1021; `.pi/extensions/background-tasks.ts`; `.pi/lib/background-task-runner.ts`

---

## Decision

Establish 1021's output capture and retrieval policy before implementing batching: combining today's full retained outputs can produce large messages, and a premature cap could conflict with full-output retrieval. A bounded window preserves completion order but can reorder completions relative to unrelated messages; verify and document that tradeoff during refinement. Pi has no delivery acknowledgement, so do not claim exactly-once. The observed cause of session summaries remains unverified.

## Problem & Goal

When many Pi background tasks finish while an agent is busy, their completion notifications can queue and be handled one at a time, consuming an agent step for each. The agent may also respond to a completion with a lengthy summary of the entire session rather than treating it as a tool result arriving during ongoing work. The owner observed this particularly with multiple successful tasks; the precise queuing and delivery mechanics need verification.

/goal Make Pi background task completions economical to process: guide agents to treat them as ordinary mid-session tool results without automatically summarizing the whole session, and aggregate queued completions where practical, particularly successful ones. Preserve actionable failures, task identities and result retrieval; verify the behavior in a live Pi session, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

Before implementation, re-check live Pi and Harnez code, installed extension coordination, recent history, and issue 1021. Determine whether batching is supported by Pi's notification and agent-turn APIs, and document any limits rather than assuming the observed scheduling cause. Verify simultaneous and staggered completions while the agent is busy, mixed success and failure, no lost or duplicated results, and normal ongoing conversation. Issue 1021 separately addresses output capture and duplicate completion dumps.

## MVP review follow-ups

Sol found no concrete MVP blocker in `4f1b2baf`. Later hardening: fix two timing tests that wait for an already-satisfied message count and therefore miss premature delivery; cap or summarize accumulated submitted IDs during sustained arrivals; add an extension-level throwing-send regression proving unpinning and retention eviction. Live-Pi and full-suite verification remain outstanding.

## MVP milestone — 2026-10-31

- Completed the partial implementation on `sprint/1047`: a fixed first-arrival window (100 ms), FIFO batches capped at 8 tasks / 8192 characters, and entries capped at 768 characters. Later arrivals do not extend the window; overflow drains one batch per poll with a fresh compaction guard check.
- Completion entries preserve task IDs and status, clip command/capture diagnostics, omit opt-out output dumps, and point to `background_task_result`. The shared guidance asks the agent to continue ongoing work without automatically summarizing the session. Existing capture/result retrieval remains available subject to ordinary retention.
- Pin finished tasks before enqueue, release after each send attempt (including throws), never retry without Pi acknowledgement, and return unsent IDs while clearing timers/pins at shutdown. Reject unusable entry/message bounds and require the message to fit an entire entry plus its envelope; JSON Schema enforces individual minima and the loader enforces the cross-field relationship.
- Verified: `HTO=0 make test-pi` — 130 passed, 0 failed/skipped; `HTO=0 go test . -run '^TestPiBackgroundTasksSpecMatchesSchema$' -count=1` — passed; `git diff --check` — passed. Deterministic tests cover collection timing, FIFO/count/character splitting, compaction, stale contexts, throws and shutdown; integration tests cover mixed outcomes, huge opt-out text, capture retention and ordinary batching pins.
- Still open: full q1 is owed after the preceding failed run and was not rerun (owner authorization required); live Pi batching/conversation verification remains outstanding. Existing no-ack/compaction coordination gaps tracked by 1047 remain. No global install/update was performed, as requested. This milestone does not establish the observed cause of session summaries or close the ticket.
- Follow-up polish: reduce redundant UI hold warnings and refine entry presentation after live-session feedback; these are outside this MVP milestone.
