# 421 — Extend telemetry for Harnez commands and feature usage analytics

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: Codex/Claude/Antigravity hook telemetry and `harnez stats`

---

## 1. Problem & Motivation

Harnez currently records many agent tool calls, but calls that execute Harnez
itself are not consistently represented as first-class feature usage events.
That prevents reliable answers to questions such as:

- Which Harnez commands and features are used by agents versus users?
- Which agent sessions invoke those features, and in which projects?
- How often do commands fail, how long do they run, and what exit statuses do
  they produce?

Extend telemetry so every invocation of the `harnez` CLI can be analyzed with
the same session, agent, project, timing, and outcome context as other tool
calls.

## 2. Technical Specification / Findings

- Capture Harnez command/subcommand and relevant non-secret arguments as a
  stable feature identifier; do not record tokens, credentials, or arbitrary
  sensitive argument values.
- Record start/end or duration, exit status, success/failure classification,
  agent identity, session identity, project/working directory, and invocation
  source (for example direct user invocation versus an agent hook/wrapper).
- Preserve correlation between a wrapped agent call and the Harnez command it
  caused, without double-counting the same execution.
- Make telemetry best-effort: telemetry failures must not alter the Harnez
  command's exit status or user-visible behavior.
- Extend query/reporting so usage can be grouped by feature, agent, session,
  project, source, outcome, and time range; retain existing `harnez stats`
  behavior for current telemetry.

Open design questions to resolve during implementation:

- Whether command identity should be stored as a normalized command path,
  Cobra command name, or both.
- How to distinguish a human shell invocation from an agent-spawned Harnez
  process when no explicit parent hook metadata is present.
- Whether stdout/stderr byte counts and selected result metadata add useful
  signal without creating privacy or storage problems.

## 3. Implementation & Verification Plan

1. **M1 — Invocation capture**: Add a single lifecycle boundary around the
   Harnez CLI that records command identity, source, session/project/agent
   context, start time, duration, exit status, and success classification.
   Add unit tests for success, failure, nested/wrapped invocation, and
   best-effort telemetry failure.
2. **M2 — Schema and migration**: Extend the telemetry storage schema with
   stable fields and an idempotent migration. Verify old databases continue to
   open and existing rows remain queryable.
3. **M3 — Reporting**: Extend `harnez stats` with filters or grouped output
   for Harnez feature usage by agent, session, project, source, outcome, and
   time range. Add deterministic fixture-based tests for aggregation.
4. **M4 — Integration coverage**: Exercise direct CLI, Codex hook/wrapper,
   Claude hook/wrapper, and at least one failing command. Verify exit status,
   duration, session context, and attribution in the stored rows and reports.
5. **M5 — Documentation and privacy review**: Document the event model,
   normalization, retention/secret-redaction rules, and migration behavior.

Acceptance criteria:

- Every `harnez` invocation produces at most one corresponding telemetry event,
  including exit status and duration, without changing command behavior.
- Events can be attributed to agent/session/project/source when that context is
  available, and missing context is represented explicitly rather than
  ambiguously.
- `harnez stats` can answer feature, agent, session, project, source, outcome,
  and time-range usage questions with tests covering the aggregations.
- Hook-generated and wrapper-generated events are correlated and not counted
  twice.
- Existing telemetry and stats tests pass, and no sensitive command data is
  persisted beyond the documented allowlisted fields.
