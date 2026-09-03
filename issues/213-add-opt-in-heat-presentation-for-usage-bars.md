# 213 — Add opt-in heat presentation for usage bars

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Issue 211; `usage-bar` in `spec/indicators.yaml`

---

## 1. Problem & Motivation

Issue 211 makes load histories and their current percentages heat-colorable,
but quota/usage bars remain monochrome. An optional matching presentation
would make the dashboard more coherent without imposing color on users who
prefer the existing quiet, monochrome display.

## 2. Technical Specification

- Add `usage-bar-presentation: monochrome | heat` to the indicator spec and
  schema. Its default must be `monochrome`.
- In `heat` mode, use the shared heat palette for the filled/partial portion
  of each usage bar and its adjacent numeric percentage. The empty portion
  remains the single spec-owned chart background; do not paint it as a second
  foreground tone.
- Retain the current full/empty glyphs, eighth-character boundary precision,
  width, wrapper setting, and textual percentage. Color is a presentation
  layer, not a new bar geometry.
- `monochrome` leaves both bar ink and percentage uncolored, while still using
  the common chart background introduced by issue 211.
- Reuse the central palette resolver; do not create a quota-bar-specific set
  of threshold colors. A future `heat-256` palette option is tracked in issue
  214.

## 3. Acceptance Criteria

- Schema/parser tests accept both values and reject unknown presentation modes.
- Rendering tests prove the filled/partial region and percentage share a
  value-derived foreground in heat mode, while the empty field retains the
  shared background and monochrome emits no heat foreground.
- Existing geometry, stripped-ANSI width, and wrapper tests remain intact.
- Visual checks at normal and compact widths show a restrained btop-style
  colored bar without distracting empty-cell stippling.
- Run focused tests, `go test ./...`, `make check`, and `make install`.
