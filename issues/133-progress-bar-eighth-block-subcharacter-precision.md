# 133 — Sub-character precision for narrow progress bars using eighth-block glyphs

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `internal/rograph/bar.go` (`RenderProgressBar`), [[078-rograph-library-shared-bar-sparkline-renderer]], `internal/usage/watch.go` (4-char bars in the All Usage compact rows, e.g. `watch.go:679-680`, `:1125`, `:1156-1157`)

## Problem

`rograph.RenderProgressBar` (`internal/rograph/bar.go:22-41`) renders a
`[filled empty]` bar at whole-character resolution:
`filledCount := int(width * usedPercent/100.0)`, using a full `█` for each
filled character and `░` for each empty one. At the narrow 4-character
width used in `harnez usage --watch`'s All Usage compact rows, this means
each character represents 25 percentage points — e.g. 24% and 1% both
round down to zero filled characters, and 26% vs 49% look identical (one
filled block). The bars work well overall (per user feedback, this is a
liked/kept feature) but lose a lot of resolution at this width.

## Scope

- Extend `RenderProgressBar` (or add a variant) to use the Unicode
  horizontal eighth-block glyphs — `▏▎▍▌▋▊▉█` (1/8 through 8/8 width) —
  for the single boundary character where the fill transitions from filled
  to empty, instead of snapping that character to fully filled or fully
  empty. Fully-filled characters before the boundary stay `█`; fully-empty
  characters after it stay `░` (or the existing empty glyph) — only the one
  boundary character gets sub-character precision.
- This turns a 4-character bar's effective resolution from 5 discrete
  states (0/4 .. 4/4) to 32 (4 characters × 8 eighths), without changing
  the bar's rendered width or the function's public contract (still
  `[width]`-character output wrapped in `[...]`).
- Keep this self-contained in `internal/rograph` so both the 4-char watch
  bars and the `rograph.MaxWidth` (10-char) bars in `usage.go` benefit
  automatically — no call-site changes needed beyond whatever the
  refactored signature requires.
- Not in scope: changing `MaxWidth`, changing which call sites use bars vs.
  sparklines, or touching `sparkline.go`'s separate `percentSparkChars`
  (vertical eighths, already multi-level) — this ticket is specifically
  about `RenderProgressBar`'s horizontal fill precision.

## Acceptance Criteria

- [ ] `RenderProgressBar` renders a partial-eighth glyph at the fill
      boundary instead of always snapping to a whole block.
- [ ] Fully-filled and fully-empty characters away from the boundary are
      unchanged (still `█` / empty glyph) — only the boundary character
      gains precision.
- [ ] Bar width and existing `[...]` wrapping contract unchanged; existing
      callers in `usage.go` and `watch.go` need no changes.
- [ ] Unit tests cover boundary rounding at multiple percentages per bar
      width (e.g. width=4 at 0%, 12.5%, 24%, 26%, 49%, 51%, 100%) asserting
      the correct eighth-block glyph appears at the boundary character.
- [ ] `go test -race ./internal/rograph/... ./internal/usage/...` passes
      clean.
