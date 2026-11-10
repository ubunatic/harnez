---
kind: sprint
date: 2026-11-10
tickets: [1211]
---

# Lean sprint 1211: `agent models --chart` cost vs capability

## Setup
- Searcher s1211 `agy:argon:low` (4m33s): located `modelsCmd`, `CostForTier`, no 2D canvas, colour helper, spec pattern.
- Developer d1211 `agy:flash38:med` (owner pick, Codex quota low). Reviewer r1211 `argon:low` one-shot.
  Web searcher w1211 `haiku:med` (1m29s) for subscription factors.

## Milestones
- M1 `e991c49d`: `internal/braillecanvas`, `spec/model_scores.yaml` + schema (1-5 star ratings per model/tier,
  per-provider subscription multipliers), `--chart --cost raw|effective`, Pareto frontier, 80-col golden. 11m53s.
- M2 (reviewer + host): weak Pareto test, `COLUMNS` ignored (<60 refusal unreachable), duplicated provider
  colours, Go defaults for scale/ticks/legend, labels overlapping lines ("argonterra"), NO_COLOR not via env.
- M3 `99f787da`: labels attached to wrong series after M2; Pareto list repeated equal-cost tiers. Fixed.
- M4 `2e6cdae8`: Codex multiplier 0.19 measured (below).

## Measurement (Codex multiplier)
Web search found no published token quotas (factor "unknown" for all). Own data instead: Codex rollouts carry
`rate_limits` and `plan_type: plus`; since the weekly drop at 09:57Z, `~/.codex/state_5.sqlite` shows 10 threads,
19.4M tokens ≈ $3.20 at the owner-pasted OpenAI list prices for 13 weekly points → ~$25/week vs $4.60/week → 0.19.
harnez's own session totals said 62.8M (≈3x over) and 0 for loom runs → ticket 1214. Week-long measurement: 1213.

## Learnings
- A cheap developer needed three review rounds for rendering; live runs (not the golden) found each regression.
- Provider-local logs beat web research for quota economics; check token sources against each other.
