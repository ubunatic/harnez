# 1232 — Add gemini-pro-orchestrator-g1-pro as a Harnez model

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 822, 942, 1203

---

## 1. Problem & Motivation
`agy models` exposes `gemini-pro-orchestrator-g1-pro` (`Orchestrator (Argon)`), an AGY two-phase Plan/Execute composite model (`plan_execute_protocol` where turns start in read-only plan mode on Argon and hand off to a cheaper execution model via `exit_plan_mode`). It is not yet registered in `spec/agent.yaml` or `spec/model_scores.yaml`, so `harnez agent` cannot address or score it.

## 2. Technical Specification / Findings
- In `agy models`, the model ID is `gemini-pro-orchestrator-g1-pro` with display name `Orchestrator (Argon)` and no `-low`/`-medium`/`-high` effort suffixes (`effort: false` in `spec/agent.yaml`, like `agy:sonnet` and `agy:opus`).
- Decide the canonical Harnez key and aliases in `spec/agent.yaml` (e.g., `agy:orchestrator` or `agy:argon-orchestrator`, plus official name resolution `gemini-pro-orchestrator-g1-pro` per issue 1203), allowed roles (`orchestrator`, `advisor`, `reviewer`), quota pool mapping in `spec/usage.yaml` (verify whether it draws from the `Gemini 4 Argon` pool or another bucket), and initial cost/ratings in `spec/model_scores.yaml`.
- Check headless `agy -p` behavior with `--model gemini-pro-orchestrator-g1-pro` (whether `exit_plan_mode` works non-interactively in a single `-p` turn or only in interactive sessions).

## 3. Implementation & Verification Plan
- Canary-test `agy --model gemini-pro-orchestrator-g1-pro -p ...` and inspect quota pool attribution.
- Add the model entry to `spec/agent.yaml`, `spec/model_scores.yaml`, and `spec/usage.yaml` if needed; update golden files (`internal/subagent/testdata/known-model-specs.golden`, `cmd/harnez/testdata/agent-models-chart-80cols.golden`) and driver tests.

**/goal** Add `gemini-pro-orchestrator-g1-pro` to the Harnez model catalog and verify dispatch and tests, or stop and report when blocked on a user decision or denied permission.
