# 155 — Define a spec-driven braille snake timeout indicator for the time gauge

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[111-per-agent-collector-pipelines-independent-cadence-and-timeout]], [[131-watch-debug-freshness-countdown-overlay]], [[150-time-gauge-color-parity-and-spec]], `internal/usage/freshness.go`, `docs/other/Spec.md`

---

## 1. Problem & Motivation

The debug time gauge currently renders its normal block-sparkline glyph even
when the scheduled refresh is overdue. That makes an expired/timeout state
look like an ordinary empty countdown rather than a distinct, active warning.

Use a finite braille “snake” depletion as the time gauge so the remaining
time is visually legible without changing the compact overlay's width.

## 2. Technical Specification / Findings

- Preserve the existing one-glyph time-gauge geometry while data is fresh and
  the next refresh is still pending.
- Render one frame from a finite braille snake sequence for the whole
  countdown. Derive its index deterministically from remaining freshness time
  so independent rows redraw in sync; do not cycle after timeout.
- Define the ordered frame sequence in a new YAML specification under `spec/`
  (for example, `spec/indicators.yaml`), not as a Go constant. It must begin
  with full braille `⣿`, follow a deterministic snake-like depletion, and
  end with empty braille `⠀`; the implementation must consume the declared
  ordering verbatim.
- Add a companion JSON Schema requiring a non-empty sequence of single-rune
  braille frames. Embed, load, and validate it with the same no-fallback
  discipline as the existing watch color spec.
- Retain the time gauge's independently specified foreground/background ANSI
  styling and its exactly one-visible-rune geometry. Do not let animation
  frames alter label padding or ANSI-stripped width.

## 3. Implementation & Verification Plan

- [x] Add the indicator YAML file and JSON Schema, with the ordered braille
  snake frames as the single source of truth.
- [x] Load the embedded sequence in `internal/usage`; map the countdown from
  full to empty without looping after timeout.
- [x] Test schema loading, exact sequence fidelity, finite depletion,
  normal countdown preservation, ANSI styling, and one-rune visible width.
- [x] Run focused usage/spec tests and `go test ./...`.
