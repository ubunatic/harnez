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
