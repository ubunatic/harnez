# 138 — Bar empty cells use a flat space under the ANSI background, not `░`

**Status**: Closed — resolved in `b6f0cbf`
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [[136-bar-ansi-background-bracket-leak-and-color-spec]] (fixed the bracket leak; this closes the remaining visual artifact from the same user screenshot review), [[133-progress-bar-eighth-block-subcharacter-precision]] (introduced `eighthBlockFill`, which this ticket modifies), `internal/rograph/options.go`

## Problem

After 136 fixed the ANSI-background bracket leak, the user compared
screenshots again and still saw more than two visual tones in the All Usage
bars. Root cause: the truly-empty trailing cells in `eighthBlockFill`
(`internal/rograph/options.go`) render `'░'` (light-shade block) even when
the whole glyph run already sits on a uniform `panel-bg` ANSI background.
`'░'` isn't "no ink" — it's its own low-density stipple pattern drawn in the
foreground color, so on top of a colored background it reads as a third,
unintended tone distinct from both the solid `'█'` fill and the flat
background. The user confirmed: they want exactly two colors — default FG
for all ink (full block and the eighth-block fractional boundary glyph
alike) and a single uniform `panel-bg` background for every cell between
`[` and `]`, matching how the Load box's CPU/GPU sparklines already look
(no separate "empty" glyph there — a sparkline always draws *some* level
glyph, never a stipple).

## Scope

- `eighthBlockFill` now takes an `emptyRune` parameter. `RenderBar` passes a
  plain space when `opts.ANSI` is true (the background wrap already covers
  the whole run, so a space renders as flat `panel-bg` with zero extra ink)
  and keeps `'░'` when `opts.ANSI` is false, so plain-text callers (e.g.
  `RenderProgressBar` used without color in `usage.go`) keep a visible bar
  shape on terminals without color support.
- Scoped to the `SubChar` eighth-block path only — the plain whole-character
  snapping path (`BarOptions.Empty` default) is untouched, so no other
  caller's rendered output changes.

## Resolution

Implemented directly (small, scoped bug fix per `docs/Git.md`/CLAUDE.md's
direct-commit allowance) rather than through a dev-subagent sprint, since
136/137 had just landed and this closes the same user-reported thread.

Updated `internal/usage/watch_test.go` assertions that hardcoded `'░'` in
expected strings (`TestBuildAllUsageBox`, `TestAllUsageBoxNarrowKeepsSecondQuotaVisible`,
`TestBuildWatchFrame_CompactShowsOnlyAllUsageAndLoad`) to expect a space
instead. Also fixed a latent test bug exposed by this change: the
column-alignment tests (`TestAllUsageBoxNarrowKeepsSecondQuotaVisible`,
`TestAllUsageBoxSecondBarColumnAlignment`) compared `strings.LastIndex`
**byte** offsets as if they were visual columns — this only worked before
because every bar glyph in use (`'█'`, `'░'`, and the eighth-block
fractional characters) happened to be a uniform 3-byte UTF-8 sequence.
Mixing in a 1-byte space broke that coincidence. Fixed both tests to
compare `utf8.RuneCountInString` up to the bracket instead of the raw byte
offset — the real rendered TUI alignment was never affected (terminal
columns are rune/display-width based, not byte-based), only the test's own
measurement method was wrong.

Verification: `go build ./...`, `go vet ./...` clean; `go test -race
./internal/rograph/... ./internal/usage/...` and full `go test ./...` (all
packages) pass. `make install` run.
