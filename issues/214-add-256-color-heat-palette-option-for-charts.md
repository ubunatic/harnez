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
