# 223 — Spec-driven usage-bar heat thresholds and a named compact-bar-width constant

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Cleanup
**Related**: [[220-add-spec-driven-braille-usage-bar-glyph-style]], `spec/indicators.yaml`, `internal/usage/indicatorsspec.go`, `internal/usage/watch.go`

## 1. Problem & Motivation

Followup audit after issue 220 (spec-driven Braille usage-bar style) and the
commit that made Braille the default. The audit was scoped to check whether
recent usage-bar changes left hardcoded styles/colors/layout/labels that
should instead live in `internal/rograph` (the charting lib) or
`spec/indicators.yaml`. Most of the recent change is clean (see audit notes
below), but it surfaced two small, genuine, pre-existing gaps worth fixing
opportunistically rather than as urgent bugs.

Audit also confirmed several apparent issues are **false positives** and
need no followup: the `UsageBarStyle` Go consts duplicated against the
`spec/schemas/indicators.schema.json` enum are the same accepted boilerplate
pattern already used by `LoadChartMode`/`LoadChartPresentation`; the bar-fill
Braille quantization (`subCharacterFill`) and the sparkline Braille dot-matrix
rendering (`brailleGlyph`/`SparklineBraille`) are genuinely different
algorithms solving different problems, not duplicated logic to unify; and
`barOptionsFromSpec`'s block/braille glyph re-derivation stays entirely
inside `internal/usage` with no rograph boundary violation.

## 2. Findings To Address

1. **Heat-band thresholds are hardcoded Go literals, not spec values.**
   `internal/usage/indicatorsspec.go`'s `heatForegroundANSI` uses a `switch`
   with literal breakpoints (`<=25` cool, `<=50` green, `<=75` yellow, else
   warm). The *color names* (`chart-cool`/`chart-green`/`chart-yellow`/
   `chart-warm`) are already spec-resolved via `colorSGR`, but the numeric
   band edges are not, which is inconsistent with this codebase's general
   pattern of pushing presentational tunables into `spec/indicators.yaml`
   (see how `style`, `braille.{full,half,empty}`, `usage-bar-presentation`,
   `load-charts`, `chart-background` are all spec-driven). Proposal: add a
   `heat-bands: [25, 50, 75]`-shaped field (exact key name TBD) under the
   relevant presentation spec block, schema-validated (ascending, in
   [0,100]), consumed by both load-chart and usage-bar heat coloring since
   they share `heatForegroundANSI`.

2. **Compact bar width `4` is a repeated unnamed literal.**
   `internal/usage/watch.go` hardcodes `Width = 4` inline at at least 7
   independent call sites (lines ~800, 802, 875, 1139, 1441, 1474, 1476 as of
   this writing — recheck line numbers before editing), unlike sibling
   layout constants in the same file (`minBoxWidth`, `loadLabelWidth`,
   `splashBarWidth = 24`). This is layout, not spec material — same category
   as `splashBarWidth` — but seven independent copies is a real maintenance
   hazard (a future width change requires hunting all call sites). Proposal:
   introduce `const compactBarWidth = 4` and replace the inline literals.

## 3. Lower-Priority / Doc-Only Follow-Ons (bundle in if convenient, not worth a separate ticket)

- `internal/rograph/options.go`'s `BarOptions.SubChar` doc comment
  (around the struct field, currently describing the sub-character gate) is
  stale: it says sub-character rendering "only applies when Fill and Empty
  are left at their defaults... custom glyphs fall back to whole-character
  snapping," but the actual gate in `RenderBar` is
  `len(SubCharacterGlyphs) > 0 || (fill=='█' && empty=='░')`, so any caller
  supplying `SubCharacterGlyphs` (which `internal/usage` always does, block
  or Braille) bypasses the fill/empty restriction the comment describes.
  Fix the comment to match the real gate.
- `subCharacterFill`'s doc comment (added in issue 220) describes the
  eighth-block case but doesn't cross-reference that `brailleGlyph`/
  `SparklineBraille` is a separate, unrelated "Braille" rendering concept in
  the same package. Add a one-line cross-reference to prevent future
  confusion between the two.

## 4. Acceptance Criteria

- Heat-band thresholds are declared in `spec/indicators.yaml`, schema
  validated, and consumed by `heatForegroundANSI` instead of hardcoded
  literals; default shipped values reproduce the current 25/50/75 bands
  byte-for-byte.
- `compactBarWidth` (or equivalent name) replaces the repeated `Width = 4`
  literals in `internal/usage/watch.go`.
- `go test ./...`, `make check`, `make install` pass.
