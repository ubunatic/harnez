# 420 — Set up Codex analytics hooks for complete harnez stats telemetry

**Status**: Open — M5 added: separate cumulative and per-turn provider token metrics
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

**M1 status (completed 2026-09-18)**: `internal/codex/events.go` lands `ParseEvent`
covering the hook envelope (`PostToolUse`) and the rollout `token_count`
shape, plus malformed/unknown-kind safety. Review found and fixed:
- `events.go`/`events_test.go` were not gofmt-clean (fixed).
- Fixture coverage was missing `PreToolUse`, a `tool_call` rollout shape, and
  session-boundary (`SessionStart`/`session_end`) kinds required by this
  milestone's own scope; added in `events_test.go`.
- A failure-shaped `PostToolUse` fixture was added, but `Success`/`ExitCode`
  are still unpopulated by `ParseEvent` — that classification is explicitly
  M2 scope ("map ... result, and failure fields"), not a regression.
- `docs/CodexHooks.md` now records the documented hook keys, observed rollout
  token schema, lifecycle guarantees, and compatibility caveats.

### M2 — Hook adapter and attribution

- Implement the Codex adapter and register it through the supported Codex
  configuration mechanism.
- Map tool name, agent, project, session, ticket, duration, result, and failure
  fields into `tool_calls` and related telemetry tables.
- Verification: isolated fixture tests cover success, failure, denial,
  interruption, duplicate delivery, and missing optional fields.

**M2 status (completed 2026-09-18)**: Codex `PreToolUse`/`PostToolUse` hooks
are installed through `config.toml`; the adapter records tool, agent, project,
session, ticket, duration, exit, output, failure, and duplicate-delivery data.
`SessionStart`, `Stop`, and `SessionEnd` now invoke the telemetry adapter for
transcript reconciliation. The live Codex 0.154.0 smoke test confirmed the
rewritten command and PostToolUse rows.

**M2 open question — still unresolved (review 2026-09-19)**:
`internal/codex/events.go` parses `success`/`exit_code`/`duration_ms` off the
Codex `PostToolUse` payload (commit `dc8d248`), but Codex's own documented
hook contract (`docs/CodexHooks.md` "Lifecycle schemas" section) only lists
`session_id`, `transcript_path`, `cwd`, `hook_event_name`, `model`,
`permission_mode`, and `turn_id` as common keys — it does not confirm
`success`/`exit_code`/`duration_ms` on a real `PostToolUse` payload. M4's
verification step (added in the prior review) asked for a captured payload to
settle this before closing the ticket; that capture never happened —
`docs/CodexHooks.md`'s "Recommended telemetry design" section now claims
`PostToolUse` is reliable for "failures, and durations" without that
verification backing it. **This should not have been closed with that bullet
unmet.** Until a real payload is captured and checked, the `PreToolUse` gear
rewrite (`harnez codex-hook` routing through `harnez exec`) remains the only
confirmed source of exit code and duration — do not drop or treat it as
redundant with `PostToolUse` parsing.

### M3 — Token and read analytics

- Parse and persist Codex token metrics at the correct turn and cumulative
  scopes, without fabricating provider values when unavailable.
- Connect native read and `harnez read` observations to existing savings and
  opportunity calculations.
- Verification: fixture data produces non-empty `harnez stats --auto --json`
  with expected counts, token fields, failure rates, and savings values.

**M3 status (completed 2026-09-18)**: `event_msg`/`token_count` records are
parsed for input, cached-input, output, reasoning, and cumulative totals.
`reasoning_output_tokens` from the live rollout schema is supported. Lifecycle
hooks attach the latest provider total to the session's latest tool row without
fabricating values when no token record exists.

### M4 — Live Codex smoke test and operational documentation

- Run a bounded real Codex session with at least one successful tool call and
  one controlled failure, then verify the session-filtered report.
- Document installation, version compatibility, failure behavior, and how to
  diagnose a session that resolves but has no telemetry rows.
- Verification: `harnez stats --auto` reports the live session and the full
  repository test/check targets pass.
- Capture a real `PostToolUse` payload from that live session and check it
  against the `success`/`exit_code`/`duration_ms` field names assumed in
  `internal/codex/events.go` (see M2 open question above); update
  `docs/CodexHooks.md` with the confirmed `PostToolUse` shape once verified,
  the same way it already documents `PreToolUse`.

**M4 status (completed 2026-09-18, verification bullet above still open)**: A
bounded real Codex session executed successfully through the installed hooks;
`harnez stats --auto` showed the resulting Codex rows and the full repository
test suite passed. A subprocess launched from an existing Harnez shell can
have a different Codex thread ID; session-filtered reports must therefore be
run from the Codex-owned environment when validating token reconciliation.
The captured-payload verification bullet was not carried out — see the M2
open question above.

### M5 — Separate cumulative and per-turn provider token metrics

- Extend `tool_calls` with nullable provider usage fields for cumulative
  `input_tokens`, `cached_input_tokens`, `output_tokens`, `reasoning_tokens`,
  and `total_tokens` values.
- Add nullable per-turn `last_*` counterparts for the same five metrics,
  sourced from Codex's `last_token_usage` object.
- Preserve `actual_tokens` for backward compatibility, but define it as the
  per-call/per-turn total when provider data is available; do not store a
  cumulative snapshot there.
- Update Codex transcript reconciliation, insertion, querying, aggregation,
  JSON output, and tests so `AVG TOKENS` uses `last_total_tokens`, while
  cumulative totals remain available for session analysis.
- Keep unavailable provider fields as `NULL`; never infer or fabricate token
  values from byte counts.
- Verification: fixture and live-session checks show distinct cumulative and
  per-turn values, and existing Claude/AGY telemetry remains unchanged.

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
