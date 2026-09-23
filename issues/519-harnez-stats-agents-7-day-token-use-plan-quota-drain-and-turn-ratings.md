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
  - **M1 delivered (47e78d1)**: before/after readings per turn in a JSONL store under
    `~/.harnez/agents/`, cache age recorded, after-reading forced fresh. Tests failed only
    on known 509 and the HTO env leak (filed as 520).
- **M2 (join and report)**: `harnez stats --agents --days N [--json]`, joining the
  telemetry DB, turn quota readings and `quota-history.jsonl`; test covers the join.
  - **Pre-Work / Required Refinements**:
    - The "before" reading uses `force=false`, so a stale cache makes the drain wrong.
      Force it fresh too, or reuse the previous turn's "after" reading when it is younger
      than ~60 s. The report marks a turn `unreliable` when a reading's cache age exceeds
      that.
    - Check that `runResume` records the same before/after pair and increments `Turn`.
  - **M2 delivered (67c80f4)**: `harnez stats --agents --days N [--json]`, before-readings
    forced fresh, resume increments `Turn`. The resume test was fixed after the single
    test run and is still unverified.
- **M3 (turn rating)**: `harnez agent rate` stores a rating per turn; shown in M2's report.
  - **Pre-Work / Required Refinements** (from the M2 output review):
    - The QUALITY column holds the drain source (`measured`/`fitted`/`unavailable`). Move
      that to its own `SRC` column; QUALITY is for M3 ratings.
    - The dev519 terra row shows TURNS 1 and tokens `—`, although the driver printed
      per-turn tokens (e.g. "368.8k new, 60.3k out"). Store the driver's per-turn tokens
      on every start/resume turn and sum them.
    - Host rows have an empty MODEL; fill it from hook/rollout data where available.
    - One fitted host row shows 43% drain. Fitting must not give a window's full drain to
      every overlapping session: split it (e.g. by new tokens) or mark it `shared`.
    - 718 rows is unreadable: default to per-model totals plus the newest 20 sessions;
      `--all` lists every session.
    - Verify the resume test fixed after M2's test run.
  - **M3 delivered (564a3d7)**: SRC column, `fitted/shared` split, newest 20 + `--all`,
    per-model totals, `harnez agent rate`.
- **M4 (proof)** — **Pre-Work / Required Refinements** (host review of M3 output):
  - NEW INPUT is wrong: terra shows 14 890 641 new and 14 481 408 cached, so "new" holds
    total input. New = input − cached (≈ 409k here). Fix in storage or report and test it.
  - `harnez agent rate --name dev519 4 "..."` fails: `session "dev519" has no recorded
    turn`, although the report shows TURNS 2. Rating must work on any session with ≥ 1
    turn, including sessions started before M1.
  - Deleted sessions vanish from the report (dev520, dev498l luna sessions are gone).
    `harnez agent delete` must keep the turn and token records for stats, or stats must
    read them from a store that delete does not touch.
  - Host MODEL is still empty and host rows are missing from per-model totals; fill the
    model from hook/rollout data (Codex rollouts carry `turn_context.payload.model`).
  - Proof runs are done by the host (leaf roles cannot run `harnez agent`).
- **M4 proof**: one short luna and one short haiku turn; reported drain per 100k new
  tokens is in the band of the hand measurement (luna ≈ 0.65, haiku ≈ 5 points).

## Done when

- M1–M4 delivered, `make test-q1` green except known 509, `docs/Telemetry.md` documents
  the command and the measured vs fitted distinction.
