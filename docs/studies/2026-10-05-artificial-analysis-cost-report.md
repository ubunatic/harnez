# Artificial Analysis Cost Report

- Reviewed: 2026-10-05
- Input: `/home/uwe/Downloads/AIAnalysis.html` (saved Artificial Analysis home/leaderboard page; capture date is not recorded in the file).
- Source page: [Artificial Analysis](https://artificialanalysis.ai/).
- Scope: cost metrics only; this is a snapshot analysis, not a refresh of `spec/agent.yaml`.

## Findings

The page reports two different task-cost measures. Its general leaderboard shows
weighted average USD cost per Artificial Analysis Intelligence Index task. The
rendered top-ten comparison is:

| Model and configuration | USD per Index task |
|---|---:|
| MiMo-V2.6-Pro | $0.13 |
| DeepSeek V4.1 Flash (max) | $0.27 |
| GPT-6.1 Sol (max) | $0.72 |
| Muse Spark 1.3 (max) | $1.60 |
| Gemini 4 Argon (high) | $1.99 |
| GLM-5.3 (max) | $2.01 |
| GPT-6 Astra (max) | $3.26 |
| Grok 4.7 (xhigh) | $3.74 |
| Claude Opus 5.5 (max with fallback) | $5.98 |
| Claude Fable 5.1 (max with fallback) | $7.63 |

These are benchmark-weighted task costs, not the cost of a fixed token request.
Artificial Analysis says the calculation accounts for input, cache-hit, cache-write,
reasoning, and answer token prices and weights tasks by its Intelligence Index.
The page identifies the index version as v4.3.2, covering ten evaluations, including
AA-Briefcase, GDPval-AA, AutomationBench-AA, Terminal-Bench 4.0, and SciCode.

The separate Coding Agent Index chart reports average pay-per-token API cost per
coding task and compares multiple harness/model combinations. It covers a different
task set and API billing path; it should not be merged with the Index task-cost values.
The saved page renders its observations as a chart without a reliably extractable
table of exact per-agent costs, so this report does not transcribe estimates from
plotted positions.

## Implications for Harnez

Harnez's `cost` field is a relative subscription-quota estimate per typical turn,
anchored at Luna = 1 and Astra = 100 ([method](../ModelResearch.md#3-reconcile)).
The file reports USD API spend for benchmark tasks. Plans, harnesses, model effort,
fallback routing, token volumes, and task definitions differ, so these values cannot
be converted directly into Harnez's quota scale or used to reorder its cost matrix.
They are useful as an independent API-spend comparison and as a signal for which
models/configurations deserve controlled, same-task cost trials.

## Limitations and next step

The HTML is a saved page with no capture timestamp; leaderboard values can change.
It presents a selected top-ten cost comparison and charts rather than a complete,
versioned raw dataset. Before changing Harnez ratings, record a fresh source capture
and compare representative configured models on matched tasks, token budgets, effort
settings, cache conditions, and provider routes. Keep subscription-quota COST and
pay-per-task API cost as separate measures unless the project deliberately changes
its cost definition.
