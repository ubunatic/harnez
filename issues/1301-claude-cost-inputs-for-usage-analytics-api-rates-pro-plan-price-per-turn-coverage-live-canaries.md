# 1301 — Claude cost inputs for usage analytics: API rates, Pro plan price, per-turn coverage, live canaries

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #1300 (parked analytics, its §2 assessment), #1302 (report, depends on this), #1215 (per-job cost record), #445 (pricing catalog), #375 (quota-history)

---

## Goal

/goal Make every Claude turn that harnez records carry a correct API cost and plan-share cost. Prove it with live canaries. Done means:
- `spec/pricing.yaml` has sourced Claude rates.
- The Claude plan price matches the owner's Pro plan.
- `turn_token_usage.cost_micros` and `turn_quota_deltas.subscription_price_share_micros` are filled for the canary runs and match a hand calculation.
- The tickets record which Claude sessions the per-turn tables cover, and why.

Stop and report when a price needs the owner's figure, or when a canary would go past the run budget below. On a loop or no progress, clear the goal and write a failure report.

## Problem

The #1300 assessment (2026-11-26) found:
- `spec/pricing.yaml` has only `gpt-5` and no Claude model rates, so `cost_micros` is null on every Claude turn.
- `spec/agent.yaml:13` prices the Claude plan as "Max 5x" at $100, but the live API reports `plan_tier: "Pro"`.
- Since 2026-11-19 the per-turn tables hold 90 Claude turns in 50 sessions and only 2 Claude quota-delta rows (both 0). The long host session bd2185a4 has no rows.
- 481 of the last 2000 lines in `quota-history.jsonl` are exact duplicates.

Re-check all of this against live code and data first.

## Plan

- **M1 (rates and plan price):**
  - Add per-model Claude rates to `spec/pricing.yaml` as a new revision. Cover Haiku, Sonnet and Opus 5.5: input, output, cache read, cache write (5-minute vs 1-hour, if they differ), and the Haiku >100k-input rate. Cite the source URL and date. Never put rates in Go.
  - Set `plan_prices.claude`, or pick the price by the live `plan_tier`, to Pro's monthly price as the owner confirms it.
  - Unit tests: a rate lookup per model, the >100k tier boundary at 100k−1, 100k and 100k+1, and an unknown model giving null rather than 0.
- **M2 (coverage and writer bugs):**
  - Find which sessions get per-turn rows (harnez agent jobs only? hooks?) and why host sessions like bd2185a4 have none. Then fix the gap, or document it as a limit #1302 must work around (reading transcripts instead).
  - Fix the duplicate quota-history writes.
  - Find why the Claude quota deltas are 0.
- **M3 (live canaries):** budgeted, read-only prompts, run with `harnez agent start --role advisor`.
  - **haiku:low:** about 5 short runs. Checks: rows written, `cost_micros` matching a hand calculation from the transcript's `usage` block, and a quota-delta row present.
  - **sonnet:low and opus:low:** at most 2 runs each, only after haiku passes. They check the higher-tier assumptions:
    - each model's rates are applied;
    - the quota % used per API dollar differs between models. `spec/agent.yaml` claims, unverified, that Haiku's subsidy is lower.
    - Weekly % moves in whole steps, so record the before and after reading, and read a "no change" result as "below resolution", not as zero.
  - Record each run's tokens, cost and quota before/after in this ticket.

## Verification

- Run the installed binary and read the rows with `sqlite3`.
- Hand-check one haiku and one opus canary against the JSONL transcript and the rates.
- `harnez usage --json` still reports `plan_tier` and the windows.

## Open facts (owner)

- The Pro monthly price, and whether it includes VAT or the price is in EUR.
- Whether to reject other machines' quota movement during canaries. Esprimo's last plan use is unknown.
