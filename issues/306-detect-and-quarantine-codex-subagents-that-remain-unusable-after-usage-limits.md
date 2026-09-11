# 306 — Detect and quarantine Codex subagents that remain unusable after usage limits

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics
**Related**: [293 — Make roadmap synthesis recoverable across quota interruptions](293-make-roadmap-synthesis-recoverable-across-quota-interruptions.md), `docs/AgenticLoop.md`

---

## 1. Problem & Motivation

Codex subagents, observed at least with Astra, can hit a usage limit and then
appear dead forever. The session may remain listed as available, but sending
additional work to it does not produce useful progress even after the quota or
usage limit should have recovered or reset. A caller that trusts the stale
session state can repeatedly route work to an unusable agent, losing time and
making delegated work appear to hang.

The workflow needs an explicit liveness and replacement policy for this failure
mode: detect a subagent that cannot resume after a usage-limit interruption,
close or otherwise retire that unusable session, permanently exclude it from
future dispatch, and create a new reusable agent for subsequent work. Recovery
of the quota alone must not silently reactivate a session whose operational
liveness has already failed.

This issue is specifically about Codex subagents and the Codex session/agent
lifecycle. It is not a request to change Claude, AGY, other agent harnesses,
generic quota handling, or the behavior of the underlying usage service.

## 2. Scope & Technical Requirements

- Establish the observable signals that distinguish a usage-limit interruption,
  a still-running session, a transient delayed response, and a session that is
  unusable for dispatch. Do not infer liveness solely from the session still
  appearing in a listing.
- Define a bounded probe or health-check policy with a timeout, retry limit,
  and terminal failure condition. The policy must avoid unbounded retries and
  must not keep sending productive work to a failed session while testing it.
- When the terminal condition is met, close/retire the Codex session through
  the supported lifecycle mechanism, record enough reason/state for operators
  to understand the replacement, and quarantine its identifier so later
  dispatch cannot select it even if a listing reports it as available.
- Create a replacement Codex agent with the required reusable configuration,
  make it eligible for later dispatch only after its creation/health check
  succeeds, and preserve the handoff context needed to continue the abandoned
  work without replaying work that may have completed.
- Make the transition idempotent. Reprocessing the same dead session must not
  create an unbounded number of replacements, close an unrelated healthy
  session, or dispatch work during the quarantine decision.
- Define how in-flight work is surfaced: the workflow must not claim work was
  completed merely because the old session stopped responding, and it must
  report whether the work is recoverable, needs replay, or needs operator
  review.
- Keep the implementation boundary Codex-only. Shared abstractions may be
  reused only when they preserve separate Codex behavior and tests; do not add
  provider-neutral lifecycle semantics or modify non-Codex agent instructions
  under this ticket.

## 3. Acceptance Criteria

- [ ] A Codex-only design documents the usage-limit, probe, timeout/retry, and
      terminal-unusable states, including the evidence required to enter each
      state.
- [ ] A session that reaches the terminal state is closed or retired, marked
      unavailable for all future dispatch, and cannot receive productive work
      after a quota reset or a stale availability listing.
- [ ] The workflow creates exactly one replacement reusable Codex agent for a
      quarantined session, verifies that replacement before dispatch, and can
      safely resume or requeue the interrupted work.
- [ ] Healthy sessions and sessions that recover within the bounded probe are
      not closed or replaced; transient failures do not cause dispatch storms.
- [ ] Repeated detection and restart attempts are idempotent and leave an
      auditable reason, old-session identifier, replacement identifier, and
      work disposition.
- [ ] Tests or deterministic fixtures cover Astra (or an equivalent Codex
      subagent) hitting a usage limit, remaining unusable after the limit
      recovers, a healthy recovery, delayed responses, duplicate detection,
      and replacement failure.
- [ ] The change includes no instructions, lifecycle behavior, or tests for
      Claude, AGY, or other non-Codex agents, and does not alter generic quota
      service behavior.

## 4. Verification Plan

1. Run the Codex lifecycle/session tests and a fixture or canary that makes a
   subagent hit the usage-limit path, then simulate the limit recovering while
   the original session remains unusable. Verify bounded detection, no further
   productive dispatch to the old identifier, retirement/quarantine, and one
   verified replacement.
2. Repeat detection after the replacement exists and confirm that no second
   replacement is created. Inspect the recorded handoff to confirm uncertain
   in-flight work is not reported as complete.
3. Exercise a healthy session, a session that responds within the probe window,
   timeout/retry exhaustion, and replacement creation failure. Verify each
   state transition and operator-visible reason.
4. Run the repository's relevant tests, `go test ./...`, and `make install` if
   the implementation changes Go code. Confirm the diff contains only the
   Codex-scoped implementation, tests, and documentation authorized by this
   ticket.

## 5. Non-Goals

- Repairing or extending the external Codex usage-limit service.
- Keeping a quota-exhausted session alive indefinitely in hopes that it will
  become usable later.
- Changing non-Codex agent providers, their prompts, their session lifecycle,
  or their dispatch instructions.
