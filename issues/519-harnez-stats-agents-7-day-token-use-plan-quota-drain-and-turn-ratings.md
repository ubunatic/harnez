# 519 — harnez stats --agents: 7-day token use, plan-quota drain and turn ratings

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Feature
**Related**: [[507-capture-subscription-quota-snapshots-at-harnez-agent-start-resume]], [[515-one-discoverable-home-for-all-harnez-telemetry-data]], [[518-measure-gpt-6-sol-plan-quota-cost]], `docs/studies/2026-09-24-model-research-plan-quota.md`, `docs/Telemetry.md`

## Problem

COST in `spec/agent.yaml` was measured by hand (jq over Codex rollouts plus
`~/.claude/harnez/usage-history/quota-history.jsonl`). Nothing in harnez joins token use
with plan-quota drain, and `harnez stats` covers tool calls only.

## /goal

A proven, repeatable setup: `harnez stats --agents --days 7` (and `--json`) lists the last
N days of agent sessions with model, turns, tokens (new input / cached / output), 5h
plan-quota drain and turn ratings, with per-model totals.

- **harnez sessions** (`harnez agent start/resume`): drain measured from quota readings
  taken right before and after each turn.
- **Host sessions** (the interactive Claude Code / Codex session the user types in, seen
  via hooks): no per-turn readings exist, so drain is fitted from quota history over time
  and labelled `fitted`.
- **Ratings**: the orchestrator rates each agent turn (1–5 + reason), e.g.
  `harnez agent rate --name w 4 "tests green"`, stored with the turn; no extra model call.

Scope: read the stores where they live today. Moving stores into one home is 515.

## Milestones

- **M1 (quota around each turn)**: record a quota reading before and after every
  `harnez agent start/resume` turn, keyed by harnez session id and turn. Covers 507's core.
- **M2 (join and report)**: `harnez stats --agents --days N [--json]`, joining the
  telemetry DB, turn quota readings and `quota-history.jsonl`; test covers the join.
- **M3 (turn rating)**: `harnez agent rate` stores a rating per turn; shown in M2's report.
- **M4 (proof)**: one short luna and one short haiku turn; reported drain per 100k new
  tokens is in the band of the hand measurement (luna ≈ 0.65, haiku ≈ 5 points).

## Done when

- M1–M4 delivered, `make test-q1` green except known 509, `docs/Telemetry.md` documents
  the command and the measured vs fitted distinction.
