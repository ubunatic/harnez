# 157 — Specify every `usage --watch` chart glyph

**Status**: Closed — resolved in 2787da7; absent chart backgrounds supported in 7da5720
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [155](155-braille-snake-timeout-indicator-spec.md), `spec/indicators.yaml`, `internal/usage`, `internal/rograph`

---

## 1. Problem & Motivation

`usage --watch` has chart glyphs hard-coded in Go: time-gauge endpoint assumptions,
quota-bar fill/empty/eighth blocks, and CPU/GPU sparkline levels. This prevents valid
spec variants and leaves the embedded specification incomplete.

## 2. Technical Specification

Declare and validate the time-gauge, usage-bar, and load-sparkline glyph sequences in
`spec/indicators.yaml`. The usage package resolves them from the embedded spec and
passes glyph options into dependency-free `rograph`; `rograph` must never load specs.
Valid braille time-gauge sequences are endpoint-defined by their first and final
declared frames, including six-dot variants.

## 3. Implementation & Verification Plan

- Extend YAML schema and strict loader validation for all chart glyph sequences.
- Remove usage-watch chart-glyph literals/fallback shadows and pass resolved glyphs to renderers.
- Test exact spec consumption, variable valid gauge lengths/endpoints, ANSI and geometry.
- Run focused tests, `go test ./...`, spec validation, `make install`, and `harnez status`.

## 4. Follow-up

`panel-bg.sgr` now accepts `null` or an empty string. In either case usage-watch
bars and CPU/GPU sparklines emit their declared foreground glyphs without an ANSI
background wrapper.
