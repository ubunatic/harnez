# 444 — Fix Dot8 renderer cell pitch for mixed glyphs

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [441](441-dot8-png-card-reader-decode-card-content-back-to-text.md), `internal/readcard/dot8_render.go`, `internal/readcard/font_glyphs.go`

---

## 1. Problem & Motivation

The Dot8 renderer advances every content rune by 3 px, but its non-Braille fallback font `Font3x5` has `CharWidth == 4`. Mixed lines therefore overlap adjacent cells and can overflow the calculated column width. This prevents deterministic Dot8 PNG decoding for issue #441 and can visibly corrupt punctuation, Markdown, and other copied characters.

## 2. /goal

Make Dot8 rendering use one consistent horizontal cell pitch for Braille cells, fallback glyphs, cursor advancement, and column sizing, so mixed-content cards render without overlap or clipping and are suitable for reliable reader round trips.

## 3. Findings

- `internal/readcard/dot8_render.go` sets `cellWidth := 3` and increments `contentX` by 3 for every rune.
- `Font3x5` is built as a 4x6 cell (`CharWidth == 4`) and can draw ink through the fourth pixel column.
- Agy Flash 3.7 independently confirmed the mismatch and its effect on adjacent-cell sampling.

## 4. Acceptance Criteria

- Use a documented, consistent pitch matching the selected fallback glyph geometry; the default fix should use the 4 px `Font3x5` pitch.
- Update Dot8 column-width calculations and content cursor advancement together.
- Add regression tests covering mixed Braille and ASCII/punctuation content, proving no adjacent-cell overlap or column-edge clipping.
- Preserve existing Dot8 legend, line-number, multi-column, compact-style, and `--dot8` behavior.
- Verify with `make install` and the repository test target; document any intentional reader limitations in issue #441.
