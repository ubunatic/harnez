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
