# 461 — Add `model` column to `tool_calls` for per-model call analytics (457 finding)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry / Data model
**Related**: [457](457-telemetry-canonical-analytics-queries-as-tests-and-live-data-quality-checks.md), [424](424-fix-telemetry-compaction-event-insertion-for-model-column.md), [445](445-add-counterfactual-api-cost-rate-cards-to-spec-and-expose-in-telemetry-reporting-and-database.md), [446](446-collect-reported-cost-fields-in-agent-responses-and-persist-to-telemetry-database.md), `spec/telemetry.yaml`

---

## Problem

`tool_calls` has no `model` column, so "calls per model" and any per-model quality or cost
analytics are impossible. `compaction_events` already has one (424). Found while building the
457 quality checks.

## /goal

`tool_calls` records the model of each call, populated at insert time where the agent exposes
it, with a versioned migration, spec-defined DDL/INSERT, and a `quality_checks` entry for the
NULL/empty rate. `stats` gains a per-model grouping through the existing `group_columns`
allowlist.

## Notes

- Sequence with 446/445, which need the same attribution for cost fields.
- Follow the 424/341 migration pattern (`BEGIN IMMEDIATE`, idempotent, legacy-fixture test).
- Re-verify against live code before starting.
