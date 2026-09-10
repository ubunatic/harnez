# 210 — Make low nonzero Braille load values visible

**Status**: Closed — resolved in eba7011: Braille load sparklines keep a visible baseline
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug
**Related**: Issue 201; `internal/rograph/options.go` (`brailleColumnLevel`)

---

## 1. Problem & Motivation

The new `braille` / `btop` load-chart mode can render a reported nonzero CPU
load as an empty chart. At 20%, the current four-dot quantization evaluates
`int((20 / 100) * 4)` to zero, producing the blank Braille cell (`⠀`). This
makes every value below 25% visually indistinguishable from actual zero.

The desired btop-style visual is a continuous baseline: even 0% should draw
the lowest Braille row (`⣀` for a two-sample cell), then rise through the
remaining rows. This sacrifices an empty visual level in favor of a smoother,
more legible timeline that never vanishes at light load.

## 2. Technical Specification

- Preserve the fixed 0--100% scale and four-dot vertical resolution for each
  half-cell.
- Every valid load value, including 0% (and invalid values normalized safely
  to the baseline), must render at least the bottom dot.
- Quantize the 0--100% range into four visible bottom-aligned heights: one,
  two, three, or four dots, with 100% reaching all four dots. Do not retain a
  blank chart level for ordinary load data.
- Preserve current clamping, ANSI, chronology, width, and block-sparkline
  behavior. This ticket changes only Braille column quantization.

## 3. Implementation & Verification Plan

- Correct `brailleColumnLevel` in `internal/rograph/options.go` without
  changing its public API.
- Add focused boundary tests for 0%, a low positive value (including 20%),
  each band transition, 100%, and out-of-range/non-finite inputs.
- Keep existing paired-cell geometry tests and assert that a zero-valued
  fixed-range pair renders the btop-style lower baseline rather than U+2800.
- Run `go test ./internal/rograph`, relevant `internal/usage` tests, then
  `go test ./...` and `make check`; record any independently reproducible
  baseline failure separately rather than weakening assertions.

---

## Implementation Plan

**Skipped — implementation already complete; only tracker closure remains.**

`brailleColumnLevel` in `internal/rograph/options.go` already implements the
btop-style visible baseline (`math.Ceil(((value-min)/(max-min))*4)` clamped into
`[1, 4]`, with NaN/-Inf/below-min normalized to the minimum and therefore to
level 1), and its comment documents the `(0,25] (25,50] (50,75] (75,100]` bands.
Coverage exists: `TestBrailleColumnLevelFixedRangeBoundaries`,
`TestRenderBrailleSparklineFixedRangeBaselineIsNotBlank` (asserts a 0%/20% pair
is not U+2800), and `TestRenderBrailleSparklineIsSafeForAllInputs`.

Remaining step: `go test ./internal/rograph ./internal/usage` passes. Run
`make check` on a clean tree (the working tree currently has unrelated in-flight
`cmd/harnez` edits that break `go vet`), then set Status to
`Closed — resolved in <commit>` and run `harnez index`.
