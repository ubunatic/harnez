# 220 — Add a spec-driven Braille usage-bar glyph style

**Status**: Open
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
