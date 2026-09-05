# 214 — Add 256-color heat palette option for charts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Issue 211; Issue 213; `spec/colors.yaml`

---

## 1. Problem & Motivation

The initial `heat` presentation uses four conservative ANSI colors. It is
terminal-safe and intentionally subtle, but a 256-color terminal can show a
smoother btop-like heat gradient. Add that richer palette as an explicit
additional choice, never as a replacement for monochrome or the basic heat
ramp.

## 2. Technical Specification

- Extend the color-presentation vocabulary with `heat-256`, alongside
  `monochrome` and `heat`. Apply it consistently wherever the applicable
  presentation mode is selected (load histories first; usage bars once issue
  213 lands).
- Define the exact 256-color ANSI palette, value-to-color interpolation or
  banding policy, boundary handling, and fallback behavior in the color spec.
  The palette must remain readable on the shared black chart background and
  follow cool-to-green/yellow-to-warm progression without high-brightness
  glare.
- A chart cell and its corresponding current percentage must resolve to the
  same palette color. Per-cell history may vary by its represented value.
- Preserve `heat` as the lower-capability/four-color choice and `monochrome`
  as a no-foreground-color choice. Do not attempt unreliable terminal-color
  capability detection; users select the presentation explicitly.

## 3. Acceptance Criteria

- Spec/schema validation accepts `heat-256` and rejects invalid modes.
- Unit tests assert representative low/mid/high 256-color SGR values,
  boundaries/clamping, balanced ANSI resets, unchanged stripped-ANSI width,
  and agreement between current label and chart color.
- Existing `monochrome` and four-color `heat` output remains covered and
  unchanged.
- Visual normal/compact watch checks confirm the 256-color gradient is
  legible but restrained on the common black chart field.
- Run focused tests, `go test ./...`, `make check`, and `make install`.

---

## Implementation Plan

### Current state (verified)

- Two independent presentation enums live in `internal/usage/indicatorsspec.go`:
  `LoadChartPresentation` (`monochrome`|`heat`, line ~50) and
  `UsageBarPresentation` (`monochrome`|`heat`, line ~63), each with a resolver
  (`chartPresentation()`, `usageBarPresentation()`) that falls back to
  monochrome, and each validated in the spec loader (~lines 345-350) plus
  `spec/schemas/indicators.schema.json`'s two `enum: ["monochrome", "heat"]`.
- `heatForegroundANSI(value float64) string` (~line 490) is the single color
  decision point. It reads `heatBands()` (3 ascending bounds, default
  `[25, 50, 75]`, issue 223) and returns `colorSGR("chart-cool"|"chart-green"|
  "chart-yellow"|"chart-warm")`.
- It has four call sites: `watchLoadBarOptions` (bar foreground),
  `sparklineOptionsFromSpec` (passed as the `opts.ForegroundANSI` *function*),
  `watchLoadPercentWithPresentation` (the label), and the usage-bar equivalents.
  Label/chart agreement is therefore automatic as long as one function stays
  authoritative.
- `spec/colors.yaml`'s `sgr` schema pattern is `^[0-9]+(;[0-9]+)*$`, which
  already accepts extended sequences like `38;5;33` — no schema change needed
  there.
- Issue 213 (usage-bar heat) is In Review and its `usage-bar-presentation` key
  exists, so this ticket's "usage bars once 213 lands" clause is unblocked.

### Steps

1. **Palette in `spec/colors.yaml`** — add eight named entries
   `chart-heat-256-1` … `chart-heat-256-8` with `sgr: "38;5;<n>"` values forming
   a cool→green→yellow→warm ramp on black (candidate 256-color indices:
   `33, 39, 44, 78, 148, 178, 208, 203` — pick final values during the visual
   check, avoiding the high-brightness 9x/2xx bright band the ticket warns about).
   Named entries keep `colorSGR` the single lookup and honour the
   "no hardcoded SGR literals in Go" rule stated in the file header.
2. **Enums** — add `LoadChartHeat256 LoadChartPresentation = "heat-256"` and
   `UsageBarHeat256 UsageBarPresentation = "heat-256"`. Update both resolvers to
   recognise the new value (they currently `EqualFold` against exactly one
   constant — switch each to a small `switch` over the known values), update both
   loader validations, and add `"heat-256"` to both schema enums.
3. **Color selection** — rename/extend the decision point to
   `heatForegroundANSIFor(presentation, value) string`, keeping
   `heatForegroundANSI(value)` as the 4-color implementation so issue 223's
   `heat-bands` behaviour and its tests stay untouched. The 256 branch maps
   `value` into one of the eight ramp entries by even division of 0–100
   (`idx = clamp(ceil(value/12.5), 1, 8)`, NaN → lowest band, mirroring
   `heatForegroundANSI`'s existing NaN-to-cool rule).
4. **Wire the four call sites** to pass the resolved presentation through:
   `watchLoadBarOptions`, `sparklineOptionsFromSpec` (the closure it assigns to
   `opts.ForegroundANSI` becomes `func(v float64) string { return
   heatForegroundANSIFor(p, v) }`), `watchLoadPercentWithPresentation`, and the
   usage-bar pair. The `presentation != Heat` early-returns in the label
   functions become `presentation == Monochrome` early-returns — that inversion
   is the one place a mistake silently drops color, so test it directly.
5. **Tests** in `internal/usage/indicatorsspec_test.go` (extend the existing
   `heatForegroundANSI` table rather than adding a parallel one):
   representative low/mid/high `38;5;` codes, every band boundary, `<0`/`>100`
   clamping, NaN; a spec-loading test that `heat-256` parses and an invalid mode
   errors; a rendering test asserting balanced `\x1b[0m` resets and unchanged
   stripped-ANSI width for a Braille and a block sparkline; and an assertion that
   `watchLoadPercentWithPresentation(heat-256, v)` and the chart cell for the same
   `v` carry the identical SGR code (the ticket's label/chart agreement criterion).
   Leave the existing `monochrome`/`heat` assertions untouched as the regression
   guard.
6. **Do not change** `spec/indicators.yaml`'s shipped
   `load-chart-presentation: heat` / `usage-bar-presentation: heat` defaults —
   `heat-256` is opt-in.
7. Focused tests → `go test ./...` → `make check` → `make install` → visual
   `harnez usage --watch` at normal and compact widths.

### Design decisions / tradeoffs

- **Eight discrete named bands, not continuous interpolation.** The ticket allows
  "interpolation or banding". Banding wins: it needs no color-space math, every
  value is spec-declared and greppable, tests can assert exact SGR strings, and
  eight steps on a 10/4-cell chart already reads as a gradient. Interpolation
  would push generated SGR literals outside `colors.yaml`, breaking that file's
  single-source-of-truth contract.
- **Do not reuse `heat-bands`.** It is validated as exactly three ascending
  values for the four-color ramp (issue 223). Overloading it for an 8-step ramp
  would either break that validation or produce an under-specified palette. Even
  division is fixed policy for `heat-256`; make it configurable only if asked.
- **Both enums get `heat-256` independently**, preserving 213's deliberate
  "load history and quota bars choose separately" split.
- **No terminal capability detection**, per the ticket — the fallback story is
  "the user picks `heat` instead".

### Risks / open questions

- Final index choices are a visual judgement; they cannot be settled from code
  and must be confirmed in a real `--watch` session against the black chart field
  before the palette values are considered final.
- `heatForegroundANSI` is currently also (indirectly) exercised by
  `internal/usage/watch_test.go`'s presentation loop over
  `[]UsageBarPresentation{Monochrome, Heat}` — extend that slice with `Heat256`
  so full-layout rendering is covered too.
- Adding a third value to two enums that are validated in four places
  (2 Go loader checks + 2 schema enums) is exactly the kind of thing that gets
  half-updated; a single test that round-trips `heat-256` through
  `parseIndicatorsYAML` catches it.

### Scope

**Small-to-medium** — one spec addition, one function split, four call-site
threadings, and a table-test extension. No new packages, no behaviour change for
existing users.
