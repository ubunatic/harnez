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

**M1 status (review 2026-09-18)**: `internal/codex/events.go` lands `ParseEvent`
covering the hook envelope (`PostToolUse`) and the rollout `token_count`
shape, plus malformed/unknown-kind safety. Review found and fixed:
- `events.go`/`events_test.go` were not gofmt-clean (fixed).
- Fixture coverage was missing `PreToolUse`, a `tool_call` rollout shape, and
  session-boundary (`SessionStart`/`session_end`) kinds required by this
  milestone's own scope; added in `events_test.go`.
- A failure-shaped `PostToolUse` fixture was added, but `Success`/`ExitCode`
  are still unpopulated by `ParseEvent` — that classification is explicitly
  M2 scope ("map ... result, and failure fields"), not a regression.
- Still open before M2: the "document the installed Codex hook/event
  payloads and lifecycle guarantees" bullet has no doc yet — no
  `docs/*Codex*Events*` or equivalent exists. Add a short doc (or a
  `docs/studies/` note) enumerating the observed hook/rollout shapes before
  building the M2 adapter on top of them.

### M2 — Hook adapter and attribution

- Implement the Codex adapter and register it through the supported Codex
  configuration mechanism.
- Map tool name, agent, project, session, ticket, duration, result, and failure
  fields into `tool_calls` and related telemetry tables.
- Verification: isolated fixture tests cover success, failure, denial,
  interruption, duplicate delivery, and missing optional fields.

**M2 open question (review 2026-09-18)**: `internal/codex/events.go` parses
`success`/`exit_code`/`duration_ms` off the Codex `PostToolUse` payload
(commit `dc8d248`), but `docs/CodexHooks.md` — sourced from OpenAI's actual
hooks docs — only documents the `PreToolUse` allow/deny envelope; it says
nothing about `PostToolUse`'s payload shape. Those three field names are
unconfirmed by analogy to Claude Code's schema, not by a real captured
payload. Until M4's live session confirms (or corrects) them, the
`PreToolUse` gear rewrite (`harnez codex-hook` routing through `harnez
exec`) remains the only confirmed source of exit code and duration —
`harnez exec` measures the command itself rather than trusting Codex's
report — so do not drop or treat the rewrite as redundant with
`PostToolUse` parsing before that's verified.

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
- Capture a real `PostToolUse` payload from that live session and check it
  against the `success`/`exit_code`/`duration_ms` field names assumed in
  `internal/codex/events.go` (see M2 open question above); update
  `docs/CodexHooks.md` with the confirmed `PostToolUse` shape once verified,
  the same way it already documents `PreToolUse`.

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
