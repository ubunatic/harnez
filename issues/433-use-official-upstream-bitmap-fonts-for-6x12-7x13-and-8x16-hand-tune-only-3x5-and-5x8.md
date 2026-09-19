# 433 — Use official upstream bitmap fonts for 6x12, 7x13 and 8x16; hand-tune only 3x5 and 5x8

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactoring

---

## Context

After 432, `internal/readcard/spec/glyphs.yaml` holds one pixel matrix per glyph and font size. The larger sizes are not real fonts:

- **7x13** was derived from the 8x16 rows 1-13 with the 8th column dropped, which clips right edges (the `A` loses its right leg pixel).
- **6x12** is the hand-tuned 5x8 moved down 2 rows into a 12-row cell (first 7 rows only), with no descender room.
- **8x16** looks like the classic VGA 8x16 but has no recorded source or licence.
- Coverage gaps follow from this: 6x12 lacks `\`, `` ` ``, `~` and 13 box/arrow symbols (`┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼ → ← ✓ ✗`); 7x13 lacks the same 13 symbols. They render as `?`.

Decision: only the small fonts (3x5, 5x8) are hand-tuned. The larger fonts should come from official, properly licensed upstream bitmap fonts.

## Candidate sources (to be confirmed)

- **Spleen** (BSD-2) provides 6x12 and 8x16 (the 12x24 is already installed at `~/.local/share/fonts/spleen/`).
- **X11 misc-fixed 7x13** (public domain) is a genuine 7x13 design.

Uncertain and to be checked: exact upstream file names and versions, licence text to record, and glyph coverage of each BDF against the charset in `glyphs.yaml`. Fetching from the network needs user approval.

## Milestones

- **M1**: Write `scripts/import-bdf-font.go` (`//go:build ignore`) converting a BDF into the `glyphs.yaml` matrices for one size. Verify: a unit test converts a small fixture BDF and compares rows.
- **M2**: Import the official 8x16, 6x12 and 7x13 fonts into `glyphs.yaml`. Keep 3x5 and 5x8 untouched. Verify: `git diff` on the 3x5 and 5x8 entries is empty.
- **M3**: Record source, version and licence per font (a `docs/` note plus a licence file if required), and update `docs/PixelFont5x8Glyphs.md` where it lists coverage gaps.
- **M4**: Regenerate the golden PNGs for the changed sizes. Verify: `make test-q1` passes and the 3x5 and 5x8 goldens are byte-identical.

## Acceptance Criteria

- 6x12, 7x13 and 8x16 matrices come from documented upstream fonts, with licences recorded.
- Every charset glyph exists in every size, or missing ones are listed explicitly with a fallback decision.
- 3x5 and 5x8 glyphs are unchanged.
- The importer is re-runnable and tested.
