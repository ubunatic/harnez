# 219 — Make usage-bar colors subtler than Braille charts

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [213](213-add-opt-in-heat-presentation-for-usage-bars.md), [211](211-adopt-btop-style-heat-colored-charts-with-a-unified-background.md), `spec/indicators.yaml`

---

## 1. Problem & Motivation

The filled portions of the `usage --watch` quota bars are visually brighter
than the Braille load charts beside them. This gives the small, secondary quota
bars disproportionate visual weight and makes the dashboard feel less calm.

## 2. Technical Specification

- Retune the usage-bar heat foregrounds to a more restrained palette that is
  visibly quieter than the Braille chart ink at equivalent terminal settings.
- Preserve value-derived color meaning, the shared chart background, and the
  existing `usage-bar-presentation: monochrome | heat` contract. This ticket
  refines `heat`; it does not add a competing presentation mode.
- Keep filled, partial, empty, wrapper, geometry, and percentage formatting
  unchanged. Only the foreground intensity/hue treatment may change.
- Apply the same subdued resolved color to the filled/partial region and its
  adjacent percentage, as established by issue 213.

## 3. Acceptance Criteria

- Visual comparison of mixed quota and Braille load panels shows usage bars as
  readable but clearly less attention-grabbing than the Braille charts.
- Heat colors remain ordered by usage value and legible on supported terminal
  themes; monochrome output remains unaffected.
- Rendering tests cover the revised resolved SGR values and preserve stripped
  ANSI width/layout assertions.
- Run focused tests, `go test ./...`, `make check`, and `make install`.

---

## Implementation Plan

### Approach

Keep one shared band function (`heatBands`, issue 223) and one shared background;
split only the *resolved color names* so usage bars draw from a subdued ramp while
Braille/load charts keep the current bright ramp. Colors stay declared in
`spec/colors.yaml` (single source of truth, per `docs/other/Spec.md` — no SGR
literals in Go).

### Steps

1. `spec/colors.yaml` — add four subdued siblings to the existing
   `chart-cool`/`chart-green`/`chart-yellow`/`chart-warm` entries, e.g.
   `usage-cool`, `usage-green`, `usage-yellow`, `usage-warm`. Simplest concrete
   values: the same base hue with the faint attribute prefixed (`"2;34"`,
   `"2;32"`, `"2;33"`, `"2;31"`). Fallback option if faint renders
   inconsistently on the user's terminal: 256-color dimmed variants
   (`38;5;24`, `38;5;65`, `38;5;136`, `38;5;131`). Decide by eyeballing
   `harnez usage --watch` before committing to one.
2. `internal/usage/indicatorsspec.go` — extract the band-selection switch in
   `heatForegroundANSI` into a small helper that takes the four color names, then
   add `usageHeatForegroundANSI(value float64) string` resolving the `usage-*`
   names. `heatForegroundANSI` keeps its current signature and `chart-*` names so
   sparkline/load-bar callers (`sparklineOptionsFromSpec`, `watchLoadBarOptions`,
   `watchLoadPercentWithPresentation`, `watch.go:1149`) are untouched.
3. Same file — repoint the three usage-side call sites at the new function:
   `usageBarOptionsWithPresentation` (~line 464),
   `watchUsagePercentWithPresentation` (~line 524), and the
   `watchUsagePercentWithDuration` path. This automatically keeps issue 213's
   "bar and its adjacent percentage share one resolved color" contract, since all
   three resolve through the same helper.
4. `internal/usage/colorsspec_test.go` — extend the required-color-name list
   (currently `chart-cool`…`chart-warm`) with the four `usage-*` names so a spec
   deletion fails loudly.
5. `internal/usage/indicatorsspec_test.go` —
   - add `TestUsageHeatForegroundBands` mirroring `TestHeatForegroundBands`,
     asserting the subdued SGRs at band edges and that they differ from the
     chart ramp at every band;
   - extend `TestUsageBarPresentationHeatCouplesBarAndPercentage` to assert bar
     and percentage carry the *usage* SGR, not the chart SGR;
   - leave `TestSelectedIndicatorOutputsHaveStableANSIVisibleGeometry` and the
     stripped-ANSI width assertions untouched — they must still pass unchanged,
     which is the proof geometry didn't move.
6. Monochrome path: no change. Verify `TestUsageBarPresentationDefaultsAndRejectsUnknownValues`
   still passes without edits.
7. Run `go test ./internal/usage/...`, then `go test ./...`, `make check`,
   `make install`. Visually confirm in `harnez usage --watch` and get the user's
   sign-off on the tuning before closing (this is a subjective-appearance ticket).

### Design decisions / tradeoffs

- **Separate color names, not a runtime dim modifier.** Declaring four extra
  named colors keeps every SGR value greppable in the spec and testable exactly,
  versus computing "chart color + faint" in Go, which would hide a literal
  `"2;"` in code and break the spec-is-source-of-truth rule.
- **Bands stay shared.** `heat-bands` (issue 223) governs *where* the color
  changes; only *which* color is emitted differs. No new spec key, no change to
  the `usage-bar-presentation: monochrome | heat` contract.
- **Faint (SGR 2) vs. 256-color.** Faint is one byte, theme-independent, and
  degrades to plain color on terminals that ignore it (acceptable — worst case
  is today's behavior). 256-color gives precise control but can clash with light
  themes. Start with faint.

### Risks / open questions

- Faint may be a no-op on some terminals, making the ticket a visual no-op there.
  If the user's terminal ignores SGR 2, fall back to step 1's 256-color set.
- "Clearly less attention-grabbing" is unverifiable by test; only the SGR values
  and unchanged geometry are testable. Requires a human look before closing.
- Check whether any other caller relies on usage bars and load bars producing
  *identical* SGRs (grep for tests comparing the two) before splitting.

### Scope

**Small** — one spec addition, one function split, three call-site repoints,
two test files.
