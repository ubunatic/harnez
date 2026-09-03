# 220 — Add a spec-driven Braille usage-bar glyph style

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [133](133-progress-bar-eighth-block-subcharacter-precision.md), [157](157-spec-driven-usage-watch-chart-glyphs.md), `spec/indicators.yaml`, `internal/rograph`

---

## 1. Problem & Motivation

Usage bars currently use block glyphs plus horizontal eighth-character
precision. Add an optional Braille-cell bar style so usage rows can match the
Braille visual language used by load charts without changing the default bar.

The requested glyph vocabulary is `full: "⣿"` and `half: "⡇"`. At the
four-cell compact width, three full cells followed by one half cell must render
as `⣿⣿⣿⡇`, representing approximately 87.5% (75% + 12.5%).

## 2. Technical Specification

- Add a documented, schema-validated `usage-bar` style option, with the
  current block/eighth-bar rendering retained as the default and `braille` as
  an opt-in value. Keep glyph definitions spec-owned rather than hard-coded.
- The Braille style declares `full`, `half`, and an empty cell glyph in the
  indicator spec. Ship `⣿` for full and `⡇` for half; use a width-one blank
  Braille cell for empty unless the final design deliberately chooses another
  spec value.
- Quantize each cell to empty, half, or full. For a width-four bar, full cells
  each represent 25 percentage points and a half cell represents 12.5 points;
  the 87.5% example is exactly `⣿⣿⣿⡇`.
- Keep wrapper behavior, configured width, ANSI/background behavior, heat vs.
  monochrome presentation, percentage text, and stripped-ANSI width unchanged.
- Resolve and validate the selected style in the usage spec loader, then pass
  its renderer options into `rograph`; `rograph` must remain spec-independent.

## 3. Acceptance Criteria

- The default block/eighth style remains byte-for-byte compatible with current
  rendering.
- A spec selecting Braille produces `⣿⣿⣿⡇` for 87.5% at width four and
  covers empty, half-boundary, full, 0%, and 100% cases.
- Schema/loader tests accept valid styles and glyphs, reject unknown styles and
  invalid/multi-rune glyph values, and do not allow the essential glyphs to be
  omitted.
- ANSI-enabled output, heat and monochrome output, wrappers, and compact/full
  widths retain their existing display-width guarantees.
- Run focused tests, `go test ./...`, `make check`, and `make install`.

## 4. Resolution

Implemented as designed:

- `internal/rograph/options.go`: generalized `eighthBlockFill` into
  `subCharacterFill`, deriving cell subdivisions from `len(partial)+1`
  instead of hardcoding 8. `eighthBlockGlyphs` trimmed from 8 to 7 runes (the
  redundant, previously-unused full-block 8th entry) so the formula reduces
  to the historical eighth-block precision with byte-for-byte identical
  output — covered by the existing eighth-block regression tests plus a new
  `TestRenderBarBrailleTwoLevelBoundary` exercising a 2-subdivision Braille
  bar directly.
- `internal/usage/indicatorsspec.go`: added `usageBarSpec.Style` (`yaml:
  "style"`, values `block`/`braille`, default `block`) and
  `usageBarSpec.Braille` (`full`/`half`/`empty`, each schema-validated via
  the existing `validateOneRune` single-rune/single-terminal-cell check, all
  three required and required pairwise-distinct whenever style resolves to
  `braille`). `barOptionsFromSpec` resolves the style and, for `braille`,
  hands `rograph.BarOptions` the Braille glyphs with a single-entry
  `SubCharacterGlyphs` (the half glyph) — reusing the generalized
  `subCharacterFill` machinery rather than a parallel code path. `rograph`
  stays spec-independent; style resolution and glyph selection live entirely
  in `internal/usage`.
- `spec/indicators.yaml` / `spec/schemas/indicators.schema.json`: added the
  `style` and `braille` fields (schema `enum`/`pattern` for `style` and each
  Braille glyph). Default spec ships `style: block` (unchanged rendering);
  a documented, commented-out `braille` opt-in example (`full: "⣿"`,
  `half: "⡇"`, `empty: "⠀"` — U+2800 BRAILLE PATTERN BLANK) is included
  inline, following the same single-active-toggle idiom as
  `usage-bar-presentation`.
- Tests added: `internal/rograph/rograph_test.go`
  (`TestRenderBarBrailleTwoLevelBoundary`, covering 0%, half-boundary,
  one full cell, the ticket's 87.5% `⣿⣿⣿⡇` example, and 100%) and
  `internal/usage/indicatorsspec_test.go`
  (`TestUsageBarStyleBrailleParsesAndRendersQuantizedCells`,
  `TestUsageBarStyleDefaultsToBlockWhenUnset`,
  `TestUsageBarStyleBrailleRejectsInvalidSpecs` — unknown style, each
  missing/multi-rune glyph, and duplicate glyphs).
- `go test ./...`, `make check`, and `make install` all pass.
