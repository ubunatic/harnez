# 155 — Define a spec-driven braille snake timeout indicator for the time gauge

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[111-per-agent-collector-pipelines-independent-cadence-and-timeout]], [[131-watch-debug-freshness-countdown-overlay]], [[150-time-gauge-color-parity-and-spec]], `internal/usage/freshness.go`, `docs/other/Spec.md`

---

## 1. Problem & Motivation

The debug time gauge currently renders its normal block-sparkline glyph even
when the scheduled refresh is overdue. That makes an expired/timeout state
look like an ordinary empty countdown rather than a distinct, active warning.

Use a cycling braille “snake” as the timeout indicator so an overdue gauge is
visually unambiguous without changing the compact overlay's width.

## 2. Technical Specification / Findings

- Preserve the existing one-glyph, block-sparkline countdown while data is
  fresh and the next refresh is still pending.
- Once the freshness interval has elapsed, render one frame from a braille
  snake sequence in place of the normal empty gauge glyph. Advance the frame
  from a deterministic time-derived index so independent rows redraw in sync.
- Define the ordered frame sequence in a new YAML specification under `spec/`
  (for example, `spec/indicators.yaml`), not as a Go constant. The initial
  candidate sequence is `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`; the implementation must consume the
  declared ordering verbatim.
- Add a companion JSON Schema requiring a non-empty sequence of single-rune
  braille frames. Embed, load, and validate it with the same no-fallback
  discipline as the existing watch color spec.
- Retain the time gauge's independently specified foreground/background ANSI
  styling and its exactly one-visible-rune geometry. Do not let animation
  frames alter label padding or ANSI-stripped width.

## 3. Implementation & Verification Plan

- [ ] Add the indicator YAML file and JSON Schema, with the ordered braille
  snake frames as the single source of truth.
- [ ] Load the embedded sequence in `internal/usage`; replace only the
  overdue/timeout gauge state with a deterministic animated frame.
- [ ] Test schema loading, exact sequence fidelity, frame cycling/wraparound,
  normal countdown preservation, ANSI styling, and one-rune visible width.
- [ ] Run focused usage/spec tests and `go test ./...`.
