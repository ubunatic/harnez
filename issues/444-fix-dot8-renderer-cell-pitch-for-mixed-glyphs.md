# 444 — Fix Dot8 renderer cell pitch for mixed glyphs

**Status**: Blocked — Dot8 experiment on hold; see 444
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

## 5. Proposed Preparation

`internal/readcard/spec/glyphs-3x5.yaml` now contains an editable matrix for every
character in `charset.yaml` (excluding procedural Braille cells). Existing upstream
Tom Thumb 3×5 matrices are the initial baseline; the previously hand-authored
special-glyph matrices are preserved. Tune these matrices here before changing the
renderer pitch. `Dot8Encode` still translates letters and digits to Braille, while
punctuation, symbols, box drawing, arrows, superscripts, and other copied characters
remain ordinary 3×5 glyphs and are the main tuning surface.

## On hold (2026-09-22)

The Dot8 experiment is parked: the token benchmark found braille characters split
under the tokenizer, so the compression is not worth the tokenizer penalty (see
`docs/studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md`),
and no agent has passed a clean reading canary (haiku failed outright, codex timed
out). This ticket is the unfixed P1 at the root of the whole chain — nothing below it
(447, 459, 441, 436, 440) should move until it lands. The CLI surface is disabled in
the meantime: `--dot8`, `--dot8-colors`, and `--dot8-pitch` are hidden and error out
unless `HARNEZ_DOT8=1` is set (`cmd/harnez/read.go`), and the exported `Dot8Encode`,
`Dot8Decode`, `Dot8RoundTripCheck`, and `Dot8CheckDocument` functions in
`internal/readcard/dot8.go` are marked `Deprecated:`. Resume once this pitch fix
lands and a clean 3x4 readability canary passes 3/3 on at least two agents.
