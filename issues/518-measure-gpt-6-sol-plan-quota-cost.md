# 518 — Measure gpt-6-sol plan-quota cost

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: `docs/studies/2026-09-24-model-research-plan-quota.md`, `docs/ModelResearch.md` (step 2b), `spec/agent.yaml` (`codex:sol` cost 50)

## Problem

`codex:sol` (gpt-6-sol) COST 50 is borrowed from gpt-5.6-sol data (98 rollouts, 20.1
5h points per 100k new tokens on ChatGPT Plus). No gpt-6-sol rollouts existed on
2026-09-24.

## /goal

A COST for `codex:sol` from gpt-6-sol's own rollouts, same method as the study
(per-rollout `used_percent` delta per 100k new tokens, × luna, × 1.6 scale).

- Wait until normal use produces ≥ 5 gpt-6-sol rollouts; don't spend turns just to measure.

## Done when

- `spec/agent.yaml` sol cost set from gpt-6-sol data (or confirmed at 50), noted in
  `docs/Models.md`.
