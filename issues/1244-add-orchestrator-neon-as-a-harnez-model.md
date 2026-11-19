# 1244 — Add Orchestrator (Neon) as a Harnez model

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1232, 1243, docs/studies/2026-11-19-two-agent-lean-sprint-1243-neon-costs.md, docs/studies/2026-11-19-session-isolation-forge-parity-and-quota-pack-story.md

---

## 1. Problem & Motivation

`agy models` exposes `gemini-pro-orchestrator-neon` (`Orchestrator (Neon)`), requested by the owner during issue 1243 (`docs/studies/2026-11-19-two-agent-lean-sprint-1243-neon-costs.md`). It is not yet registered in `spec/agent.yaml` or `spec/model_scores.yaml`, so `harnez agent` cannot dispatch or score it (issue 1232 covers only `gemini-pro-orchestrator-g1-pro` / `Orchestrator (Argon)`).

## 2. Technical Specification / Findings

- Live `agy models` output (`2026-11-19`) includes `gemini-pro-orchestrator-neon\tOrchestrator (Neon)` alongside `gemini-pro-orchestrator-g1-pro\tOrchestrator (Argon)` and `gemini-4-neon-{low,medium,high,xhigh}`.
- Check whether `gemini-pro-orchestrator-neon` takes `-low`/`-medium`/`-high` effort flags (`effort: false` vs effort tiers in `spec/agent.yaml`), define its canonical Harnez key/aliases (aligning naming conventions with issue 1232), allowed roles (`orchestrator`, `reviewer`, `advisor`), quota bucket mapping in `spec/usage.yaml`, and initial FrontierCode 2.0-aligned cost/tier entry in `spec/model_scores.yaml`.
- Verify headless `agy --model gemini-pro-orchestrator-neon -p ...` behavior (including `plan_execute_protocol` / `exit_plan_mode` compatibility in non-interactive runs).

## 3. Implementation & Verification Plan

- Run a read-only canary (`agy --model gemini-pro-orchestrator-neon -p ...`) and inspect quota attribution in `harnez usage`.
- Register the model in `spec/agent.yaml` and `spec/model_scores.yaml`, update golden files (`internal/subagent/testdata/known-model-specs.golden`, `cmd/harnez/testdata/agent-models-chart-80cols.golden`), and add driver/spec unit tests (`make lint` and `make test-q1`).
