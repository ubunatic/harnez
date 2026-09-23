# 516 — Measure agy plan-quota cost on Google Pro

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research
**Related**: `docs/ModelResearch.md` (step 2b, "COST unit"), `docs/studies/2026-09-24-model-research-plan-quota.md`, `spec/agent.yaml`, [[507-capture-subscription-quota-snapshots-at-harnez-agent-start-resume]], [[515-one-discoverable-home-for-all-harnez-telemetry-data]]

## Problem

COST is now plan quota per turn (luna = 1, astra = 100), measured for ChatGPT Plus and
Claude Pro on 2026-09-24. agy (Google Pro) had no usable data: no per-turn token record,
quota readings only every ~20–30 min, one agent session record. So `agy:flash37/38`,
`agy:sonnet` and `agy:opus` keep old guesses (4 / 16 / 32), although the user reports
flash feels expensive on the subscription.

## /goal

A measured COST for every `agy:*` row on the same scale, or a documented reason why it
can't be measured yet.

- Find agy's token source (if any) and whether agy's Claude models draw from a separate
  Google pool.
- Cheapest likely method: a few bounded `agy:*` turns with a quota reading right before
  and after each (depends on 507 or a manual `harnez usage` read), nothing else running on
  the Google plan.

## Done when

- `spec/agent.yaml` agy costs updated from measurements, snapshot in `docs/Models.md`,
  short study in `docs/studies/`.
