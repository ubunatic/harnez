# 446 — Collect reported cost fields in agent responses and persist to telemetry database

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `pkg/telemetry/`, `pkg/agent/`, `harnez usage`
**Note**: capture moves to the shared package in 495; agents store in their own records and hand to telemetry only when it is active (`docs/HarnezComponents.md` §8.9).

---

## 1. Problem & Motivation

Several agent harnesses and API endpoints (e.g. direct LLM calls or API-metered proxy responses) return explicit `cost`, `total_cost`, or billing metadata in response JSON / session completion payloads. When available, Harnez should capture and persist these reported costs rather than discarding them.

## 2. Desired Behavior & Goal

`/goal`: Ingest and persist explicit `cost` fields from agent responses and session telemetry into the Harnez database alongside token usage.

- Parse `cost`, `total_cost`, `currency`, and related provider billing fields from agent responses and session summaries.
- Store reported cost fields in the telemetry database schema.
- Expose actual reported costs in `harnez usage` and `harnez stats` reports when present (distinguishing between reported actual cost and spec-based counterfactual cost).

## 3. Implementation Plan

1. Update agent response parsing and session record ingestion to extract cost fields.
2. Ensure database schemas and migration scripts support storing reported cost values.
3. Add unit and integration tests verifying cost extraction and database persistence.
