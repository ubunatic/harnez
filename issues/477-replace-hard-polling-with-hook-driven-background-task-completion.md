# 477 — Replace hard polling with hook-driven background task completion

**Status**: Open

**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics
**Related**: #479

---

## 1. Problem & Motivation

Host agents must not repeatedly poll background workers with fixed sleeps or
short status checks. Hard polling wastes turns, delays useful work, and can
encourage agents to stop or misclassify a worker simply because no output has
appeared yet. In a recent handoff, the host fell back to polling instead of
waiting for a completion signal.

Codex needs an explicit lifecycle mechanism that captures every background task
and tells the host when work has completed, failed, or requires attention.

## 2. Proposed Direction

- Investigate the Codex/harness hook surface for background-task creation,
  completion, failure, interruption, and process exit events.
- Add a tracked-task registry or equivalent lifecycle channel so every spawned
  task has an identity, owner, command/session, and terminal state.
- Deliver completion notifications to the host session through the supported
  hook/event mechanism, including the worker's final response or a concise
  status summary.
- Make host guidance explicit: do not use hard polling for tracked background
  tasks; wait on the completion signal or use a supported blocking/wait API.
- Ensure cleanup and failure paths emit events and prevent zombie tasks.
- Add an integration test or deterministic harness fixture proving that a host
  can launch a background task and receive completion without polling.

Do not remove useful status inspection for diagnosis; the requirement is to
replace periodic polling as the normal completion mechanism with event-driven
notification.

## 3. Verification

- Identify and document the hook/API used, including behavior on success,
  failure, timeout, cancellation, and host disconnect.
- Test multiple concurrent tasks and verify each completion is attributed to
  the correct host/session.
- Verify no orphaned process remains after completion or cancellation.

/goal

Provide a reliable event-driven background-task lifecycle for Codex hosts so
completion is reported automatically and agents never need hard polling to
discover that work is done.

## Epic note (#479)

Prerequisite for the `--async` mode in #476: detached workers need an event-driven completion channel so hosts never poll and no zombie worker is left behind.
