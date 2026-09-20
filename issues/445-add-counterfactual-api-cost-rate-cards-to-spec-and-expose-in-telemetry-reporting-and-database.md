# 445 — Add counterfactual API cost rate cards to spec and expose in telemetry reporting and database

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `spec/`, `harnez usage`, `harnez stats`

---

## 1. Problem & Motivation

Harnez tracks token counts across agent runs, but local agent sessions (routed via subscription tiers like Codex Plus, Claude Pro, or AGY Consumer) report $0 direct spend. Users need counterfactual API cost calculations to understand equivalent pay-as-you-go spend, prompt-caching savings, and comparative model economics across sessions.

## 2. Desired Behavior & Goal

`/goal`: Define model rate cards in `spec/` for major provider models and expose calculated counterfactual API costs in Harnez telemetry reporting commands (`harnez usage`, `harnez stats`) and telemetry database storage.

- Add model rate cards (uncached input, cached input, output per 1M tokens) in `spec/` covering:
  - **Gemini**: `flash` / `pro` (3.7 / 3.8)
  - **Claude**: `fable`, `opus`, `sonnet`, `haiku` (latest)
  - **Codex / OpenAI**: `astra`, `sol`, `terra`, `luna`
- Store calculated counterfactual costs alongside token records in the database.
- Present estimated equivalent cost and prompt-caching savings in `harnez usage` and `harnez stats`.

## 3. Implementation Plan

1. Create/extend `spec/pricing.yaml` (or `spec/models.yaml`) with input/cached/output pricing schemas for the specified models.
2. Update telemetry ingestion/DB storage to record calculated counterfactual costs for agent sessions.
3. Update `harnez usage` and `harnez stats` formatters to surface cost metrics.
4. Add unit tests for pricing calculations and spec validation.
