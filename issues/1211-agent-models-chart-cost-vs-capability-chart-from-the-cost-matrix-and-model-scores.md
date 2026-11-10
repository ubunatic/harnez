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
- 2026-11-10 owner (supersedes the host's skill-mark sum): a Pareto chart built only from harnez data, no
  external data to refresh. FrontierCode is an example and a hint to sanity-check our numbers.
  - Capability: define our own assessment. harnez has no score or star rating today (only Go/TUI/SQL skill
    marks). Start simple: one capability rating per model and effort tier in spec (with schema), e.g. a
    1-5 star scale with half steps; finer per-category ratings can come later.
  - Cost: the relative cost matrix in `spec/agent.yaml` (luna:low = 1) is the raw API cost.
  - Subscription multiplier: new per-provider factor in spec (none exists today) that turns raw API cost
    into effective cost on the owner's subscription. A flag toggles raw vs effective, e.g.
    `--chart --cost raw|effective` (default decided in the plan).
- 2026-11-10 host (decide mode): initial ratings and multipliers are seeded by the developer from existing
  harnez evidence (picks, skill marks, cost tiers), each value marked "owner estimate, unverified";
  FrontierCode reference scores may only be used to check the ordering, not copied. Seeded values are
  listed in the ticket for owner review before close. X = cost (log), Y = rating, one series per model
  across tiers, colored by provider, Pareto frontier marked. `--chart --json` is refused.

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

## M1 delivered (e991c49d): canvas, spec + loader, --chart/--cost, golden

Seeded multipliers (owner review pending): codex 0.25, claude 1.0, agy 0.33, jules 1.0, local 0.

## M2 Pre-Work / Required Refinements (host review + r1211 argon:low)

1. Pareto test must fail on a wrong implementation (e.g. `points[:1]`): assert the full frontier list for a
   fixture with a dominated point, a tie on cost and a tie on rating.
2. Width: honor `COLUMNS` when not a TTY; test the <60 refusal through the cobra command, not only
   `renderModelsChart`.
3. Spec: drop `provider_colors`; take provider colours from `spec/colors.yaml`. No Go defaults for scale,
   ticks, frontier style, legend provider list, fallback multiplier/colour: derive from spec (scale from
   `scores.Scale`, legend from providers present). Missing multiplier = validation error, not 1.0.
4. Labels: `IsOccupied` must also check dots; a label that cannot be placed without collision is
   dropped from the canvas (still named in the Pareto/legend text). No concatenated labels like
   "argonterra"; regenerate the golden and add a collision unit test.
5. NO_COLOR test goes through `agentListColorEnabled` with the env var set.
6. Effective mode axis label must not claim "luna:low = 1" (state the base, e.g. "raw luna:low = 1").
