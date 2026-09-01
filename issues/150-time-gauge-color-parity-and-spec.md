# 150 — Give the debug time gauge graph-color parity and an independent color spec

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[147-debug-overlay-gauge-does-not-count-down]], [[133-progress-bar-eighth-block-subcharacter-precision]], `spec/colors.yaml`, `internal/usage/freshness.go`

---

## 1. Problem & Motivation

The compact watch debug overlay's time/freshness gauge uses only its foreground
glyph color. Other watch graphs use a background color, so the time gauge does
not visually belong to the same graph system and is harder to distinguish from
ordinary label text.

## 2. Technical Specification / Findings

- Render the time-gauge glyph with the same background treatment as the other
  watch graphs.
- Add explicit, independently named foreground and background colors for the
  time gauge in `spec/colors.yaml`.
- The time-gauge foreground/background entries must remain distinct from the
  foreground/background colors used by other graphs, even when initial values
  intentionally match; do not encode this by implicitly reusing another
  graph's color definition.
- Resolve both colors from the specification rather than hard-coding ANSI SGR
  sequences in the usage renderer.

## 3. Implementation & Verification Plan

- [ ] Extend the color specification and its validation with separate
  time-gauge foreground/background entries.
- [ ] Apply both configured colors to the compact debug time gauge without
  changing label width, ANSI-stripped output, or countdown timing.
- [ ] Add rendering tests that assert the time gauge's foreground and
  background SGR sequences independently from other graph styling.
- [ ] Verify focused usage/spec tests and `go test ./...`.
