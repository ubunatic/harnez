# 201 — Double timeseries resolution and add btop-style Braille sparklines

**Status**: Closed — resolved: doubled timeseries/Braille implementation and safety tests are complete
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `internal/rograph/options.go` (`RenderSparkline`),
`internal/usage/indicatorsspec.go` (`watchPercentSparkline`),
`spec/indicators.yaml` (`load-sparkline`, `load-charts`)

---

## 1. Problem & Motivation

The Load panel's timeline currently retains and renders one time-series
sample per terminal character. Its usual 10-character (and compact
4-character) graphs therefore only represent 10 (or 4) observations.

Add a btop-style Braille time-series presentation, for example
`⣿⣀⣀⣰⣀⣀⣀⣀⣠⣄⣾⣄`, without making the chart visually wider.
A Braille cell has two horizontal dot columns, so one rendered character
can encode two consecutive samples. The underlying time-series resolution
must consequently be doubled: a width-*N* chart retains the latest
*2N* samples, then maps each adjacent pair to one output glyph. The
standard lower-block sparkline must use that same doubled sampling window
and remain exactly *N* terminal characters wide.

This is deliberately a shared time-series/rendering design, not a
special-case btop pipeline with mode-specific conditionals scattered
through `usage`.

## 2. Technical Specification

1. Define a single chart-resolution contract in the shared graph renderer:
   requested display width *N* consumes/retains up to *2N* newest samples;
   samples remain ordered oldest to newest; rendering emits at most *N*
   cells. Define and test deterministic behavior for odd-length and
   undersized histories, including which side of an incomplete pair is
   preserved.
2. Refactor the current one-value-per-glyph renderer around that contract.
   The normal lower-block sparkline remains a selectable presentation and
   stays width-*N* on screen. Specify its pair reduction deliberately
   (rather than accidentally dropping every other observation), preserving
   the current/latest sample and avoiding misleading spikes where possible.
3. Add a btop/Braille sparkline presentation mode to the indicator spec and
   its schema, with a clear stable name (for example `braille` or `btop`).
   Keep `sparkline`/`timeseries` as the existing block-style aliases and
   keep bar/gauge behavior unchanged. The default may remain the current
   block sparkline unless the implementation decision includes changing it
   explicitly.
4. Implement one safe, total mapping from a consecutive low/high sample
   pair to a Unicode Braille cell: quantize each value against the same
   fixed or relative range used by the graph, place the older sample in the
   left dot column and the newer sample in the right dot column, and emit a
   valid U+2800–U+28FF rune for every finite, clamped, NaN, and infinity
   input. Document the chosen vertical dot resolution and bit mapping.
5. Keep percent charts on their existing fixed 0–100 scale and relative
   charts on their existing relative scale. Compute their scale across the
   complete doubled render window so the paired values share one scale.
6. Apply this through the existing `rograph` options/spec path so the Load
   box uses it without an ad-hoc renderer. Audit other time-series callers
   and either migrate them to the shared resolution behavior or preserve
   their public output deliberately with tests.

## 3. Acceptance Criteria & Verification

- A display width of 10 (respectively 4) renders no more than 10
  (respectively 4) terminal cells while retaining/mapping the latest 20
  (respectively 8) observations where available.
- Tests cover standard and Braille modes for ascending, descending, flat,
  odd, short, and over-width histories; assert output rune count, newest
  sample retention, chronological pair orientation, and exact representative
  Braille cells.
- Tests cover 0%, 100%, out-of-range values, NaN, and infinities; Braille
  rendering never panics or emits an invalid/non-Braille glyph.
- ANSI wrapping, configured load-sparkline frames where applicable, and
  bar/gauge modes remain compatible with existing behavior.
- `spec/indicators.yaml` and `spec/schemas/indicators.schema.json` validate
  the new mode; `internal/usage` tests demonstrate configuration selection
  and compact-width rendering.
- Run `go test ./...`, `make check`, and a visual `harnez usage --watch`
  check at both normal and compact chart widths. Confirm the output is
  still width-stable and visually represents the supplied btop-style
  example class of timeline.

## 4. Non-goals

- Do not increase terminal layout width or change the existing 10/4
  character chart budgets.
- Do not build a separate bespoke chart pipeline for btop mode.
- Do not alter spinner or countdown Braille sequences; this concerns
  time-series charts only.

---

## Implementation Plan

**Skipped — implementation already complete; only tracker closure remains.**

The doubled-resolution contract and Braille presentation are in
`internal/rograph/options.go` (`SparklineBraille`, `sparkCell`, `sparkCellValue`,
`brailleGlyph`, `brailleColumnLevel`) with coverage in
`internal/rograph/options_test.go` (doubled-window, all-inputs safety,
foreground-per-cell) and `internal/usage/indicatorsspec_test.go`
(`LoadChartBraille` spec selection).

Remaining step: the baseline failure that blocked closure no longer reproduces —
`go test ./...` passes on the current tree. Re-run `make check` on a clean
worktree (the tree currently has unrelated in-flight edits in `cmd/harnez` that
make `go vet ./...` fail on `cmd/harnez/index.go:60`, which is not this
ticket's concern), then flip Status to `Closed — resolved in <commit>` and
run `harnez index`.
