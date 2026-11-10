# 1211 — agent models --chart: cost vs capability chart from the cost matrix and model scores

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 711 (cost matrix), 1210 (costly-model guard), 218/191 (terminal charts)

---

## 1. Problem & Motivation

Owner request (2026-11-10): `harnez agent models` lists cost and skill marks per model as a table, but
there is no view that shows cost against capability at a glance, as in the FrontierCode 2.0 "Score vs
Cost" chart (https://cognition.com/frontiercode): log-scale cost on X, score on Y, one line per model
across effort tiers, colored by harness (Claude Code, Codex, ...).

## 2. Goal

/goal `harnez agent models --chart` draws a terminal chart: X = relative cost from the `spec/agent.yaml`
cost matrix (log scale), Y = a capability score, one series per model with its effort tiers as points,
labeled, colored by provider. Scores come from a spec file with schema and a recorded source per value
(e.g. FrontierCode), never Go defaults; models without a score are listed below the chart, not plotted.
Golden tests pin the rendering at a fixed width. Stop and ask the owner before choosing the score source
or copying third-party numbers into the repo. On a loop or no progress, clear the goal and write a
failure report here.

## 3. Acceptance

- Chart fits the terminal width (narrow fallback or clear message below a minimum width); no color when
  output is not a TTY or `NO_COLOR` is set.
- Each plotted point traces to a spec value with its source.
- `--chart` with `--json` is refused or emits the series data (decide and record).

## 4. Notes / Unverified

- Unverified: whether FrontierCode's per-tier scores can be used and cited; the owner decides the score source.
- The table's relative cost (luna:low = 1) differs from the chart's USD per rollout; keep relative units
  and label the axis accordingly.
- Existing Braille chart code (218, 191) may be reusable; check before writing new rendering.

## Reference scores (FrontierCode 2.0, owner screenshot 2026-11-10; NOT used by the chart)

Approximate best score per model, read off the chart at https://cognition.com/frontiercode (avg USD per
rollout in brackets): GPT-6 Luna ~22 ($0.17), GPT-6.1 Sol ~39 ($1.0), GPT-6 Astra ~43 ($6.7), Haiku 5.5 ~28
($0.37), Sonnet 5.5 ~41 ($20), Opus 5.5 ~45.5 ($13), Fable 5.2 ~49 ($17; ~45.5 at $55), Gemini 4 Carbon ~40
($4.1), Gemini 4 Argon ~26 ($6.5). Kept for comparison only.

## Decision

- 2026-11-10 owner: the chart uses harnez's own numbers, not FrontierCode's.
- 2026-11-10 host (decide mode): "own numbers" = the per-model skill marks already shown by `agent models`
  (Go/TUI/SQL: `+` = 2, `~` = 1, `-` = 0; sum 0-6; `?` = no data, model listed below the chart). Mapping
  lives in spec with schema, not Go. X = relative cost per tier from `spec/agent.yaml` (log scale), one
  series per model across its tiers, colored by provider. `--chart --json` is refused with a clear error.
  Owner may swap the score source later; keep it one spec mapping.

## Searcher findings (s1211, agy:argon:low, 4m33s)

- `agent models`: `modelsCmd` `cmd/harnez/agent_cmds.go:142-258` (flags `--names --picks --md --json --offline`;
  JSON :193-201 via `agent.ModelRows`; table :202-236). Cost per tier: `subagent.CostForTier`
  `internal/subagent/driver.go:265`, used in `KnownModelEntriesE` :395-430; source `spec/agent.yaml:127-150`.
- Charts: `internal/rograph` (`RenderSparkline`, `RenderBar`) and `internal/assess/braille.go` draw only
  one-row sparklines/bars; no 2D canvas, log axis, multi-series or labels. A new 2D Braille canvas is needed.
- Color: `agentListColorEnabled` `cmd/harnez/agent.go:329-344` (always/never/auto, NO_COLOR, TTY); width
  via `term.GetSize` (`cmd/harnez/tui.go:107-119`); palette `spec/colors.yaml`.
- Spec pattern: `spec/disable.yaml` + `spec/schemas/disable.schema.json`, test
  `internal/disable/disable_test.go:28-78`. Golden: `TestModelPicksGolden` `cmd/harnez/agent_picks_test.go:13-30`.
