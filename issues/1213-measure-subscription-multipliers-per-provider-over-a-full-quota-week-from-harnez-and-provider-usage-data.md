# 1213 — Measure subscription multipliers per provider over a full quota week from harnez and provider usage data

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1211 (chart, `subscription_multipliers` in spec/model_scores.yaml), 1214 (token overcount)

---

## Problem

1211 seeded multipliers as owner estimates; only Codex has one sample (2026-11-10: Plus, 19.4M tokens from
`~/.codex/state_5.sqlite` ≈ $3.20 at API list price for 13 weekly points → 0.19). Providers publish no token
quotas, so the factor can only be measured.

## Goal

/goal A `harnez` command (or documented procedure) computes, per provider, effective/raw = plan price per week ÷
(API-priced tokens per weekly quota point × 100), from the quota timeline (`harnez usage history timeline`) and
the provider's own token logs (Codex state DB / rollout `token_count` events with `rate_limits`; Claude
transcripts). API prices and plan prices live in spec with source and date. Run it over one full week for
Claude and Codex; Gemini/agy stays "unknown" if no token log exists. Update the spec values with the measured
source. Stop and ask on plan-price choices. On a loop or no progress, clear the goal and write a failure report.

## Notes

- Exclude usage not billed on the subscription (taia uses the OpenAI API directly).
- Depends on correct token counts (1214) or reads provider logs directly.
