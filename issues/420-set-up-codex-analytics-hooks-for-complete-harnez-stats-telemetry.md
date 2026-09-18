# 420 — Set up Codex analytics hooks for complete harnez stats telemetry

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics / Infrastructure
**Related**: #116, #179, #227, #296, #416, `harnez stats --auto`

---

## 1. Problem & Motivation

Codex tool calls are not currently visible to the current-session analytics
pipeline. In a Codex session, `harnez stats --auto` resolves a valid session ID
but reports no `tool_calls` rows or heartbeat records, while the all-time report
contains historical Codex rows from other sessions.

This prevents Codex sessions from contributing reliable data to `harnez stats`,
including tool frequency, failure rates, feedback scores, token counts,
distillation savings, read opportunities, and session heartbeats. The existing
Claude/AGY-oriented lifecycle integration does not appear to cover the Codex
environment's tool events end to end.

## 2. Technical Specification / Findings

- Identify the Codex lifecycle surfaces available for `PreTool`, `PostTool`,
  `ToolCall`, session start, and session end events.
- Add or repair Codex-compatible hooks/adapters so every eligible tool call is
  attributed to the active Harnez session, agent, project, and optional ticket.
- Preserve the existing telemetry contract used by `harnez rate`, `harnez exec`,
  and `harnez stats`; do not create a parallel Codex-only store.
- Capture provider-reported token usage where available, including input,
  cached-input, output, reasoning, total, and per-turn/cumulative values.
- Record successful, failed, denied, and interrupted calls consistently, with
  correct failure classification and duration.
- Ensure read-related calls can populate token savings and opportunity metrics
  when the Codex environment invokes native file tools or `harnez read`.
- Make hook behavior safe when an event payload is incomplete, duplicated, or
  unavailable on a particular Codex CLI version.

## 3. Implementation & Verification Plan

### M1 — Codex event and payload discovery

- Document the installed Codex hook/event payloads and lifecycle guarantees.
- Add a canary or fixture that emits representative PreTool, ToolCall,
  PostTool, failure, and session-boundary events.
- Verification: the canary identifies the active session and records a complete
  event sequence without requiring a real model run.

### M2 — Hook adapter and attribution

- Implement the Codex adapter and register it through the supported Codex
  configuration mechanism.
- Map tool name, agent, project, session, ticket, duration, result, and failure
  fields into `tool_calls` and related telemetry tables.
- Verification: isolated fixture tests cover success, failure, denial,
  interruption, duplicate delivery, and missing optional fields.

### M3 — Token and read analytics

- Parse and persist Codex token metrics at the correct turn and cumulative
  scopes, without fabricating provider values when unavailable.
- Connect native read and `harnez read` observations to existing savings and
  opportunity calculations.
- Verification: fixture data produces non-empty `harnez stats --auto --json`
  with expected counts, token fields, failure rates, and savings values.

### M4 — Live Codex smoke test and operational documentation

- Run a bounded real Codex session with at least one successful tool call and
  one controlled failure, then verify the session-filtered report.
- Document installation, version compatibility, failure behavior, and how to
  diagnose a session that resolves but has no telemetry rows.
- Verification: `harnez stats --auto` reports the live session and the full
  repository test/check targets pass.

## 4. Acceptance Criteria

- A current Codex session with tool activity produces `tool_calls` rows visible
  through `harnez stats --auto`.
- Codex calls appear under the correct agent and project breakdowns in
  `harnez stats`.
- Token metrics are populated from provider data when supplied and remain
  explicitly unavailable when not supplied.
- Success, failure, denial, interruption, and duration metrics are accurate and
  do not double-count duplicate lifecycle events.
- Read-related token savings and opportunity metrics use the existing Harnez
  telemetry schema and calculations.
- Existing Claude and AGY telemetry behavior remains intact.
- Tests and a bounded live smoke test demonstrate the complete Codex path.

## 5. Verification Guidance

Use a temporary or dedicated Codex session and inspect both the raw telemetry
rows and `harnez stats --auto --json`. Compare the current session ID resolved
by Harnez with the session ID stored in each row. Run the relevant unit,
integration, and repository check targets after implementation.
