# 136 — Bar ANSI background leaks onto brackets; consolidate colors into spec/

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [[133-progress-bar-eighth-block-subcharacter-precision]] (introduced the regression this fixes), `internal/rograph/options.go` (`RenderBar`, `RenderSparkline`), `internal/usage/watch.go` (`formatCPULine`/`formatGPULine` vs. the All Usage bar call sites), [[132-watch-spec-driven-superscript-hotkeys]] (`spec/actions.yaml` — this ticket adds a sibling `spec/colors.yaml` to the same `spec/` directory it bootstrapped)

## Problem

User screenshots comparing the "All Usage" box (built on 133's bars) against
the "Load" box (`cpu`/`gpu` lines) show the All Usage bars looking
visually "wild" and inconsistent, while the Load box's bars read as clean
and uniform. Root cause, found by comparing the two render paths in
`internal/usage/watch.go`:

- **Load box** (`formatCPULine`, `formatGPULine`, `watch.go:863-903`) calls
  `rograph.PercentSparkline(series, ...)` and interpolates the result
  *inside* literal brackets: `fmt.Sprintf("%s [%s] %s%s", label, sparkline,
  ...)`. Only the sparkline glyph itself gets the `\x1b[100m` background —
  the `[`/`]` characters are plain text outside the ANSI wrap.
- **All Usage box** (the four `rograph.RenderBar(..., BarOptions{Width: 4,
  SubChar: true, ANSI: true})` call sites added in issue 133, e.g.
  `watch.go:711-712`, `:1168`, `:1199-1200`) instead builds the bracketed
  string `"[" + glyphs + "]"` *inside* `RenderBar` and wraps the **whole
  bracketed string**, brackets included, in the ANSI background
  (`internal/rograph/options.go`, `RenderBar`'s final `if opts.ANSI` block).

The user also asked to use the *same* color as the CPU/GPU graphs (both
currently default to the same `"100"` SGR code, so the color value itself
already matches — the visible difference is purely the bracket-inclusion
bug above) and to move color values out of hardcoded Go strings into
`spec/`, consistent with how issue 132 moved the hotkey mapping into
`spec/actions.yaml`.

## Scope

1. **Fix the bracket leak**: `RenderBar`'s `ANSI`/`BackgroundANSI` wrap
   must apply only to the glyph portion (`glyphs` in the current
   implementation), not to `left`/`right` (the `[`/`]` wrapper) or the
   optional percent label. Match `PercentSparkline`'s behavior exactly:
   brackets and any trailing text stay outside the ANSI escape sequence.
2. **Color spec**: Add `spec/colors.yaml` (+ `spec/schemas/colors.schema.json`)
   alongside `spec/actions.yaml`, defining named ANSI SGR background codes
   (e.g. a `panel-bg` or similar name for `"100"`) rather than the literal
   string `"100"` hardcoded as a Go default in two places
   (`internal/rograph/options.go`'s `RenderSparkline` and `RenderBar`
   defaults). Embed via `//go:embed` per `docs/Spec.md`, following the same
   loading/validation pattern issue 132 established in
   `internal/usage/actionsspec.go` (a sibling `colorsspec.go` or an
   extension of the existing spec-loading machinery — avoid duplicating
   the embed/parse/validate boilerplate if it can reasonably be shared).
   `RenderSparkline`/`RenderBar`'s `BackgroundANSI` default should resolve
   from this spec instead of a hardcoded `"100"` literal.
3. Verify the actual rendered color is now visually consistent between the
   Load box and All Usage box bars (same SGR code, same bracket
   treatment) — this was already numerically the same default before this
   fix; the fix is about *where* the escape sequence starts/ends, not
   changing the color value itself, unless the color spec introduces a
   deliberate, documented change.

## Acceptance Criteria

- [ ] `RenderBar`'s ANSI background wraps only the glyph portion; brackets
      and any percent label render outside the escape sequence, matching
      `RenderSparkline`/`PercentSparkline`'s existing behavior.
- [ ] `spec/colors.yaml` + `spec/schemas/colors.schema.json` exist, and the
      `"100"` SGR default used by both `RenderSparkline` and `RenderBar`
      is sourced from this spec, not a hardcoded Go string literal in two
      places.
- [ ] Visual/unit test confirms the escape-sequence boundaries: e.g. a
      rendered bar string with `ANSI: true` matches
      `"[" + "\x1b[100m" + <glyphs> + "\x1b[0m" + "]"`, not
      `"\x1b[100m" + "[" + <glyphs> + "]" + "\x1b[0m"`.
- [ ] All existing callers (All Usage bars, Load box sparklines) keep
      working with no behavior change beyond the bracket fix.
- [ ] `go test -race ./internal/rograph/... ./internal/usage/...` passes
      clean.
- [ ] `harnez status` confirms tracker sync after filing/closing.
