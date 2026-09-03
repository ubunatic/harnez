# 210 — Make low nonzero Braille load values visible

**Status**: In Progress
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

## 2. Technical Specification

- Preserve the fixed 0--100% scale and four-dot vertical resolution for each
  half-cell.
- Zero (and invalid values normalized to zero) must remain an empty Braille
  column.
- Every finite positive value in the fixed range must render at least the
  bottom dot. Quantize positive values into four visible bands, with 100%
  reaching all four dots.
- Preserve current clamping, ANSI, chronology, width, and block-sparkline
  behavior. This ticket changes only Braille column quantization.

## 3. Implementation & Verification Plan

- Correct `brailleColumnLevel` in `internal/rograph/options.go` without
  changing its public API.
- Add focused boundary tests for 0%, a low positive value (including 20%),
  each band transition, 100%, and out-of-range/non-finite inputs.
- Keep existing paired-cell geometry tests and add an assertion that a
  low-positive fixed-range sample is not U+2800.
- Run `go test ./internal/rograph`, relevant `internal/usage` tests, then
  `go test ./...` and `make check`; record any independently reproducible
  baseline failure separately rather than weakening assertions.
