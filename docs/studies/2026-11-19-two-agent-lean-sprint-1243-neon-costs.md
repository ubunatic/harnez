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
- **M1 (Onboard `agy:neon`)**: Delivered (`c9582b7d`) by `dev-1243` (`haiku:med`, 20m6s, 254k tokens). `make install` + touched tests (`internal/subagent`, `cmd/harnez`) passed; live canary `canary-neon` (`neon:med`) replied `OK` in 9s; `rev-1243-m1` (`neon:med`, 1m38s, 201k tokens) returned **PASS** with minor M2 pre-work (`xhigh` rating, `ModelCost` test, `costly_models` check).
- **M2 (FrontierCode 2.0 cost/tier/ratings update + `agy` `developer` role)**: Delivered (`a9bade32`) by `dev-1243` (`haiku:med`, 6m43s, 3.3M tokens incl. cached, auto-compacted at 85k). `make test-q1` green (`ok ubunatic.com/harnez/scripts/gotest-summary (cached)`, JS `pass 132, fail 0`). `rev-1243-m2` (`neon:med`, 2m15s, 301k tokens) returned **PASS WITH MINOR FINDINGS** (`agy:neon:med` 108→109 rounding; `agent_cost_test.go` stale `wantCost: "unknown"` + missing `neon` costly cases).
- **M3 (M2 review refinements)**: Pending dispatch.
