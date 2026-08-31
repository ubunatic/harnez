# 133 — Sub-character precision for narrow progress bars using eighth-block glyphs

**Status**: Closed — resolved in `5fd2efb`, `c7c5fdf`, `0666758`
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `internal/rograph/bar.go` (`RenderProgressBar`), `internal/rograph/sparkline.go` and `internal/rograph/options.go` (`RenderSparkline`'s existing `\x1b[100m` ANSI-background convention this ticket must match), [[078-rograph-library-shared-bar-sparkline-renderer]], `internal/usage/watch.go` (4-char bars in the All Usage compact rows, e.g. `watch.go:679-680`, `:1125`, `:1156-1157`)

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

## Background Color Requirement

The bar's empty portion must **not** rely on a dim/shaded foreground glyph
for visual distinction (e.g. a muted-looking `░` character choice) — that
approach doesn't compose with real terminal themes and is the thing this
requirement replaces. Instead, wrap the whole bar in the **same ANSI
background sequence already used for sparklines**: `RenderSparkline`
(`internal/rograph/options.go:114-121`) wraps output in
`"\x1b[" + BackgroundANSI + "m" + out + "\x1b[0m"`, defaulting
`BackgroundANSI` to `"100"` (bright-black) when `ANSI` is true — this is
the "CPU/GPU graph" panel background referenced above. `RenderBar`/
`RenderProgressBar` currently have no equivalent option at all. Add the
same `ANSI`/`BackgroundANSI` fields (or reuse `BarOptions` to add them) so
bars get a real background color, consistent with sparklines, instead of
leaning on glyph dimness.

## Acceptance Criteria

- [x] `RenderProgressBar` renders a partial-eighth glyph at the fill
      boundary instead of always snapping to a whole block.
- [x] Fully-filled and fully-empty characters away from the boundary are
      unchanged (still `█` / empty glyph) — only the boundary character
      gains precision.
- [x] Bar width and existing `[...]` wrapping contract unchanged; existing
      callers in `usage.go` and `watch.go` need no changes unless they
      opt into the new background option.
- [x] `RenderBar`/`RenderProgressBar` gain an ANSI-background option
      matching `RenderSparkline`'s `\x1b[100m`-style wrap (same default
      SGR code, same opt-in `ANSI`/`BackgroundANSI` shape) rather than
      relying on the empty glyph's visual dimness for contrast.
- [x] The 4-char All Usage bars in `watch.go` opt into the new background
      option so they visually match the sparklines' existing panel
      background.
- [x] Unit tests cover boundary rounding at multiple percentages per bar
      width (e.g. width=4 at 0%, 12.5%, 24%, 26%, 49%, 51%, 100%) asserting
      the correct eighth-block glyph appears at the boundary character.
- [x] `go test -race ./internal/rograph/... ./internal/usage/...` passes
      clean.

## Resolution

Consolidated `RenderProgressBar` (`internal/rograph/bar.go`) to delegate
to `RenderBar`'s options-based path (`internal/rograph/options.go`),
adding two new `BarOptions` fields: `SubChar` (eighth-block boundary
precision, applied only when using the default `█`/`░` glyphs) and
`ANSI`/`BackgroundANSI` (background wrap matching `RenderSparkline`'s
convention, default SGR `"100"`). `RenderProgressBar` always passes
`SubChar: true` (that's the ticket's core behavior change to its default
output); the ANSI background stays fully opt-in via `BarOptions`, so
`RenderProgressBar`'s signature and existing callers who don't switch to
`RenderBar` are unaffected. The four 4-char All Usage bar call sites in
`internal/usage/watch.go` were switched from `RenderProgressBar` to
`RenderBar(..., rograph.BarOptions{Width: 4, SubChar: true, ANSI: true})`.
Commits: `5fd2efb` (eighth-block precision), `c7c5fdf` (watch bar test
updates for the new precision), `0666758` (ANSI background wiring in
watch.go).
