# 1243 — Onboard Gemini 4 Neon and update model costs/tiers from FrontierCode 2.0

**Status**: Closed — M1-M3 delivered: onboarded agy:neon (live canary OK in 9s), allowed developer role on agy models, recalibrated costs/tiers from FrontierCode 2.0, and tightened agent_cost_test assertions; make test-q1 and touched Go suites passed.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 974 (onboarded GPT-6.1 Terra), 504 (onboarded GPT-6 Sol and Luna), 1233 (terra:high efficient-frontier developer), [spec/agent.yaml](../spec/agent.yaml), [spec/model_scores.yaml](../spec/model_scores.yaml), [spec/pricing.yaml](../spec/pricing.yaml)

---

/goal Onboard Google Gemini 4 Neon (`agy:neon`) into `spec/agent.yaml`, `spec/model_scores.yaml`, `spec/pricing.yaml`, and model docs/tests, and update model costs, ratings, and tiers across existing models based on the FrontierCode 2.0 leaderboard data, verified with a live `agy` canary and passing `make test-q1`; or stop and report when blocked on an owner decision (aliases, role placement, or cost/multiplier calibration) or if `gemini-4-neon` is unavailable in the installed `agy` CLI.

## 1. Problem & Motivation

Google's Gemini 4 Neon model ranks #3 overall on the FrontierCode 2.0 leaderboard (`46.2%` score at `$4.38` cost/rollout on `high`), outperforming `Gemini 4 Carbon` (`42.6%` at `$5.06`), `Gemini 4 Hydrogen` (`38.9%` at `$4.95`), and `Gemini 4 Argon` (`27.6%` at `$6.51`). Harnez currently configures `agy:argon` and `agy:hydrogen` in `spec/agent.yaml` and `spec/model_scores.yaml`, but has no `agy:neon` entry. Additionally, existing model costs, ratings, and tier placements in `spec/agent.yaml`, `spec/model_scores.yaml`, and `spec/pricing.yaml` should be recalibrated against the FrontierCode 2.0 benchmark and cost-per-rollout numbers.

## 2. Technical Specification / Findings

### FrontierCode 2.0 Leaderboard Data (Snapshot 2026-11-19)

Relevant rows from the FrontierCode 2.0 leaderboard (`#`, `Model`, `Harness`, `Score`, `Pass rate`, `Flag rate`, `Cost / rollout`, `Output tokens`):

- **Google / Antigravity CLI & Devin CLI**:
  - `3` `Gemini 4 Neonhigh` (`Antigravity CLI`): `46.2%` | `50.8%` | `0.0%` | `$4.38` | `85.6k`
  - `3` `Gemini 4 Neonxhigh` (`Antigravity CLI`): `44.8%` | `49.2%` | `0.0%` | `$7.67` | `128.0k`
  - `3` `Gemini 4 Neonmedium` (`Antigravity CLI`): `38.2%` | `42.2%` | `0.0%` | `$2.17` | `45.9k`
  - `3` `Gemini 4 Neonlow` (`Antigravity CLI`): `27.4%` | `30.6%` | `0.0%` | `$0.96` | `21.0k`
  - `7` `Gemini 4 Carbonhigh` (`Antigravity CLI`): `42.6%` | `46.9%` | `0.3%` | `$5.06` | `127.6k`
  - `7` `Gemini 4 Carbonmedium` (`Antigravity CLI`): `40.9%` | `45.2%` | `0.0%` | `$4.49` | `114.4k`
  - `7` `Gemini 4 Carbonlow` (`Antigravity CLI`): `31.5%` | `34.6%` | `0.0%` | `$7.72` | `51.6k`
  - `10` `Gemini 4 Hydrogenxhigh` (`Antigravity CLI`): `38.9%` | `43.2%` | `0.0%` | `$4.95` | `153.7k`
  - `10` `Gemini 4 Hydrogenhigh` (`Antigravity CLI`): `38.2%` | `42.2%` | `0.0%` | `$2.40` | `86.0k`
  - `10` `Gemini 4 Hydrogenmedium` (`Antigravity CLI`): `28.2%` | `31.6%` | `0.0%` | `$0.90` | `37.8k`
  - `10` `Gemini 4 Hydrogenlow` (`Antigravity CLI`): `10.5%` | `11.9%` | `0.0%` | `$0.35` | `15.0k`
  - `17` `Gemini 4 Argonhigh` (`Devin CLI`): `27.6%` | `30.6%` | `0.0%` | `$6.51` | `148.6k`
  - `17` `Gemini 4 Argonmedium` (`Devin CLI`): `24.4%` | `27.1%` | `0.0%` | `$5.69` | `123.9k`
  - `17` `Gemini 4 Argonlow` (`Devin CLI`): `22.4%` | `24.9%` | `0.0%` | `$3.98` | `63.3k`
- **Anthropic / Claude Code**:
  - `1` `Fable 5.2xhigh`: `50.2%` | `54.7%` | `0.0%` | `$17.73` | `156.0k`
  - `1` `Fable 5.2max`: `48.2%` | `52.7%` | `0.1%` | `$55.87` | `417.8k`
  - `1` `Fable 5.2high`: `46.2%` | `50.4%` | `0.0%` | `$8.40` | `76.0k`
  - `1` `Fable 5.2medium`: `38.8%` | `42.4%` | `0.0%` | `$3.10` | `28.4k`
  - `1` `Fable 5.2low`: `35.3%` | `38.6%` | `0.0%` | `$2.62` | `23.9k`
  - `2` `Opus 5.5max`: `47.3%` | `51.8%` | `0.0%` | `$12.82` | `201.9k`
  - `2` `Opus 5.5xhigh`: `44.7%` | `48.8%` | `0.1%` | `$5.18` | `88.1k`
  - `2` `Opus 5.5high`: `36.3%` | `39.9%` | `0.0%` | `$2.42` | `41.2k`
  - `2` `Opus 5.5medium`: `31.4%` | `34.5%` | `0.0%` | `$1.70` | `30.1k`
  - `2` `Opus 5.5low`: `14.8%` | `16.5%` | `0.0%` | `$0.75` | `14.2k`
  - `8` `Sonnet 5.5max`: `42.2%` | `46.4%` | `1.2%` | `$19.94` | `514.7k`
  - `8` `Sonnet 5.5xhigh`: `36.5%` | `40.0%` | `0.0%` | `$3.31` | `80.4k`
  - `8` `Sonnet 5.5high`: `22.9%` | `25.4%` | `0.0%` | `$1.07` | `27.8k`
  - `8` `Sonnet 5.5medium`: `15.0%` | `16.7%` | `0.0%` | `$0.48` | `14.4k`
  - `8` `Sonnet 5.5low`: `9.7%` | `10.9%` | `0.0%` | `$0.37` | `11.3k`
  - `13` `Haiku 5.5max`: `28.9%` | `31.9%` | `0.0%` | `$0.37` | `205.1k`
  - `13` `Haiku 5.5xhigh`: `22.9%` | `25.4%` | `0.0%` | `$0.23` | `127.7k`
  - `13` `Haiku 5.5high`: `19.3%` | `21.5%` | `0.0%` | `$0.14` | `81.0k`
  - `13` `Haiku 5.5medium`: `14.1%` | `15.8%` | `0.1%` | `$0.10` | `57.6k`
  - `13` `Haiku 5.5low`: `10.6%` | `11.8%` | `0.0%` | `$0.07` | `39.4k`
- **OpenAI / Codex**:
  - `4` `GPT-6.2 Astramax`: `44.2%` | `48.2%` | `0.0%` | `$5.97` | `46.0k`
  - `4` `GPT-6.2 Astraxhigh`: `44.1%` | `48.5%` | `0.0%` | `$4.29` | `28.6k`
  - `4` `GPT-6.2 Astrahigh`: `43.6%` | `47.7%` | `0.0%` | `$3.95` | `24.5k`
  - `4` `GPT-6.2 Astramedium`: `42.2%` | `46.5%` | `0.0%` | `$2.47` | `14.4k`
  - `4` `GPT-6.2 Astralow`: `38.1%` | `42.2%` | `0.0%` | `$1.42` | `8.2k`
  - `6` `GPT-6.2 Solmax`: `43.8%` | `48.2%` | `0.0%` | `$1.29` | `38.1k`
  - `6` `GPT-6.2 Solxhigh`: `39.8%` | `43.9%` | `0.0%` | `$0.98` | `26.4k`
  - `6` `GPT-6.2 Solhigh`: `39.7%` | `43.9%` | `0.0%` | `$0.88` | `22.4k`
  - `6` `GPT-6.2 Solmedium`: `38.3%` | `42.2%` | `0.0%` | `$0.69` | `16.5k`
  - `6` `GPT-6.2 Sollow`: `33.1%` | `36.6%` | `0.0%` | `$0.40` | `9.8k`
  - `9` `GPT-6.1 Solmax`: `39.6%` | `43.0%` | `0.0%` | `$1.03` | `41.5k`
  - `9` `GPT-6.1 Solhigh`: `35.1%` | `38.3%` | `0.0%` | `$0.65` | `22.4k`
  - `9` `GPT-6.1 Solxhigh`: `33.0%` | `36.1%` | `0.0%` | `$0.69` | `24.8k`
  - `9` `GPT-6.1 Solmedium`: `30.3%` | `33.0%` | `0.0%` | `$0.49` | `15.7k`
  - `9` `GPT-6.1 Sollow`: `25.7%` | `28.5%` | `0.0%` | `$0.34` | `10.0k`
  - `22` `GPT-6 Lunamax`: `22.7%` | `25.1%` | `0.0%` | `$0.17` | `79.7k`
  - `22` `GPT-6 Lunaxhigh`: `18.4%` | `20.4%` | `0.0%` | `$0.13` | `50.6k`
  - `22` `GPT-6 Lunahigh`: `16.5%` | `18.5%` | `0.0%` | `$0.12` | `42.2k`
  - `22` `GPT-6 Lunamedium`: `16.1%` | `17.9%` | `0.1%` | `$0.09` | `31.4k`
  - `22` `GPT-6 Lunalow`: `4.0%` | `4.5%` | `0.0%` | `$0.02` | `6.5k`

### Uncertainties to Resolve at Implementation Time

- Confirm exact `agy` CLI model slug and supported effort tiers (`low`, `med`, `high`, `xhigh`, `max`) for Gemini 4 Neon via installed `agy`.
- Confirm how FrontierCode 2.0 rollout costs and scores map to Harnez's relative `cost` / `cost_by_tier` (`spec/agent.yaml`), `ratings` (`spec/model_scores.yaml`), `spec/pricing.yaml`, and `roles` tier lists (`lean`, `smart`, `best`).
- Decide whether `agy:carbon` or `GPT-6.2` variants should be tracked separately or only existing Harnez models + `agy:neon` are updated in this ticket.

## 3. Implementation & Verification Plan

### M1: Onboard `agy:neon` (`gemini-4-neon`) — Delivered (`c9582b7d`)
- Onboarded `agy:neon` (`low`/`med`/`high`/`xhigh`) in `spec/agent.yaml`, `spec/model_scores.yaml`, `docs/Models.md`, golden files, and `agy_test.go`/`driver_test.go`.
- `make install` + `go test ./internal/subagent/... ./cmd/harnez/...` passed; live canary `canary-neon` (`neon:med`) replied `OK` in 9s; `rev-1243-m1` (`neon:med`) returned **PASS**.

### M2: Recalibrate Model Costs, Ratings, and Tiers from FrontierCode 2.0

#### Pre-Work / Required Refinements (from M1 review & owner decision)
1. **Allow `developer` role on `agy:*` models (owner decision 2026-11-19)**: add `developer` to `agy:neon` (and other `agy` models as appropriate) in `spec/agent.yaml` and update/remove the `agy:*` non-developer assertion in `internal/subagent/agentspec_test.go:83-84`.
2. **`agy:neon` M1 review gaps**: add `xhigh` rating for `agy:neon` in `spec/model_scores.yaml`, add `ModelCost` assertions for `agy:neon` tiers in tests, and decide/update `costly_models` for high-cost `neon` tiers (`high`/`xhigh`) if appropriate.

#### M2 Core Work — Delivered (`a9bade32`)
- Added `developer` role to `agy:neon`, `agy:hydrogen`, `agy:argon`, `agy:flash38` and updated `internal/subagent/agentspec_test.go`.
- Added `xhigh: 4.0` rating for `agy:neon` in `spec/model_scores.yaml`, `ModelCost` assertions, and `costly_models` for `agy:neon:{high,xhigh}`.
- Recalibrated `cost` / `cost_by_tier` in `spec/agent.yaml` (`luna`, `sol`, `haiku`, `sonnet`, `fable`, `opus`, `argon`, `hydrogen`, `neon`) from FrontierCode 2.0 (`round_half_up($ / 0.02)`), replaced `argon:high` with `neon:high` in `roles.reviewer.models.best`, and synced `docs/Models.md`, `docs/HarnezAgentArchitecture.md`, `docs/practices/ModelRoles.md`, `AgenticLoop`, command docs, and tests (`make test-q1` green).
- `rev-1243-m2` (`neon:med`) returned **PASS WITH MINOR FINDINGS** (2 minor items queued as M3).

### M3: M2 Review Refinements
#### Pre-Work / Required Refinements (from `rev-1243-m2`)
1. Fix `agy:neon:med` rounding inconsistency: `$2.17 / $0.02 = 108.5` rounds half-up to `109` (currently `108` in `spec/agent.yaml`, `docs/Models.md`, and `agentspec_test.go` if asserted).
2. Fix `cmd/harnez/agent_cost_test.go:96-100`: replace stale `wantCost: "unknown"` on `agy:argon:high` (`326`) and `claude:fable:high` (`420`) with the real cost strings, and add `costly_models` refusal cases for `agy:neon:high` (`219`) and `agy:neon:xhigh` (`384`).
3. Run `make install` and `make test-q1`, then commit with `git commit -m "fix(spec): round agy:neon:med to 109 and tighten agent_cost_test assertions (issue 1243 M3)"`.

## Decision
- **2026-11-19 (Host)**: Split #1243 into M1 (onboard `agy:neon` quickly) and M2 (recalibrate all model costs, ratings, tiers, and docs from FrontierCode 2.0) per owner instruction.
- **2026-11-19 (Owner)**: `let agy take "developer" too` — allow `developer` role on `agy:*` models (removing the restriction in `agentspec_test.go:83-84`).
