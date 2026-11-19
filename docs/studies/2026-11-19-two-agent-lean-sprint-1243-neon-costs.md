---
kind: sprint
ticket: 1243
date: 2026-11-19
branch: sprint/1243
---

# Two-Agent Lean Sprint: 1243 — Onboard Gemini 4 Neon & Update Costs/Tiers (FrontierCode 2.0)

## 1. Searcher Audit (`search-1243`, `claude:haiku:med`)
- `agy models` lists `gemini-4-neon-{low,medium,high,xhigh}` (no `max`, no bare `gemini-4-neon`). `internal/subagent/agy.go` passes `--model gemini-4-neon --effort <tier>`.
- `agy:carbon` is absent from `agy models` and the repo.
- `agentspec_test.go:83-84` forbids `developer` role on `agy:*` models.
- FrontierCode 2.0 `GPT-6.2 Astra/Sol` do not match `spec/agent.yaml` IDs (`gpt-6-astra`, `gpt-6.1-sol`, `gpt-6.1-terra`); requires checking `~/.codex/models_cache.json` in M2.

## 2. Milestones
- **M1 (Onboard `agy:neon`)**: Pending dispatch.
- **M2 (FrontierCode 2.0 cost/tier/ratings update)**: Pending M1.
