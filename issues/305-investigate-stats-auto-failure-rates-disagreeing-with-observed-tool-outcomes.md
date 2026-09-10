# 305 — Investigate `stats --auto` failure rates disagreeing with observed tool outcomes

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [181 — failure-only rating policy](181-narrow-harnez-rate-to-failure-cases.md), [215 — telemetry tool-note reclassification](215-llm-backfill-and-reclassification-mode-for-telemetry-tool-notes.md), [294 — cross-agent success hooks](294-investigate-cross-agent-post-edit-success-hooks-for-the-harnez-rate-pipeline.md), [296 — stats distillation measurements](296-always-compute-distill-savings-for-stats-even-when-harnez-distill-autopipe-is-off.md)

---

## 1. Problem & Motivation

`harnez stats --auto` is intended to inform the current session about tool
reliability. Its failure-rate report does not match the outcomes visible in a
recent Loom session (2026-09-10): it reported 100% failure for `make-test`,
`go-test`, `rg`, and `exec_command`, although the final `make test`,
`go test -race ./...`, `go vet`, and other checks passed. It also reported
high failure rates for `apply_patch` and `harnez-index`; those partly match
real intermediate failures.

This makes the automatic feedback actively misleading: a session may be told
that reliable tools are failing, while genuine transient failures and later
successful retries are not represented in an understandable way. Determine
whether the defect is in collection, outcome classification, aggregation, or
the documented meaning of the metric.

The session included context compaction and delegated agents. The visible
transcript may therefore be incomplete, and `stats --auto` may intentionally
aggregate attempts outside the visible transcript. The observation is a
reproduction lead, not proof that every displayed row is wrong.

## 2. Investigation Scope, Evidence, and Uncertainties

Investigate the complete path from tool outcome capture through the
`stats --auto` query, grouping, rendering, and feedback text. Compare raw
attempts and their exit/result metadata with displayed rates for shell and
native tools.

Test these hypotheses without assuming any one of them:

- failed and successful attempts are aggregated incorrectly, such as a
  failure-only numerator with the wrong denominator or an incorrect retry
  merge;
- successful calls are classified as failures because of exit-code, stderr,
  wrapper, timeout, or post-processing semantics;
- `make-test`, `go-test`, `rg`, and `exec_command` are aliases or tool labels
  whose calls are double-counted or mapped to the wrong outcome;
- telemetry from delegated agents or pre-compaction history is merged into
  the current session incorrectly, or is correctly included but not disclosed;
- `--auto` intentionally measures subjective/unconfirmed failure reports
  rather than command success, but its wording or documentation fails to say so.

Preserve the distinction between a process result, an agent quality judgment,
a manual failure rating, and telemetry-ingestion failure. Do not use the
visible transcript alone as ground truth while compaction and subagent
aggregation remain unresolved.

## 3. Scope Boundaries

Own diagnosis and a narrowly justified correction to the `stats --auto`
contract, classifier, aggregation, or documentation. Do not expand this into
a broad telemetry schema redesign, a new cross-agent success-hook project,
distillation changes, or unrelated command-execution fixes. Avoid changing
historical data until semantics and migration/backfill behavior are explicit.
If the data cannot distinguish these cases, document that limitation and the
smallest additional evidence needed.

## 4. Acceptance Criteria

- [ ] Identify the actual failure mode, or rule out each hypothesis testable
      from available telemetry; record what remains unobservable.
- [ ] Define numerator, denominator, attempt identity, retry handling,
      time/session/project scope, and treatment of unknown, cancelled,
      timed-out, wrapper, delegated, and compacted calls.
- [ ] Reproduce the mismatch with a hand-computable fixture or canary covering
      successes, deliberate failures, retries, aliases/wrappers, and, where
      supported, delegated-agent records.
- [ ] Make `stats --auto` agree with the defined semantics, or document those
      semantics prominently and change misleading output. Unknown or absent
      visible history must not silently become failure.
- [ ] Add regression coverage for successful final checks alongside failed
      intermediate attempts, including compaction/subagent aggregation
      boundaries. Preserve manual-rating behavior.
- [ ] Run relevant telemetry/stats tests, `go test ./...`, `go vet ./...`, and
      a bounded live or recorded `stats --auto` verification. If code changes
      are made, run required install and smoke checks; this filing changes no
      source code.

## 5. Verification Guidance

Start with the smallest reproducible dataset. Inspect raw telemetry rows and
the exact query inputs used by `stats --auto`, rather than whole session logs.
Run one success, one failure, and a successful retry for each affected label;
compare the expected ratio with table and JSON output. Repeat across session
boundaries and delegated-agent records if supported.

Confirm the output says whether it reports process outcomes, agent/manual
ratings, or an aggregate across sessions. Verify incomplete transcript,
compaction, and missing-result cases are unknown or explicitly included under
the documented contract, never silently failures. Record exact command,
version, and data scope for live confirmation.
