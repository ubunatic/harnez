# 433 — Use official upstream bitmap fonts for all non-default sizes; keep only the default 5x8 hand-tuned

**Status**: Closed — upstream fonts imported; remaining upstream glyph gaps documented in third_party/fonts/README.md
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

Decision: only the default font (5x8, `DefaultFont`) stays hand-tuned. Every non-default size (3x5, 6x12, 7x13, 8x16) should come from an official, properly licensed upstream bitmap font, with no hand-crafted pixel art or derivations.

## Candidate sources (to be confirmed)

- **Spleen** (BSD-2) provides 6x12 and 8x16 (the 12x24 is already installed at `~/.local/share/fonts/spleen/`).
- **X11 misc-fixed 7x13** (public domain) is a genuine 7x13 design.
- **3x5**: Spleen has nothing this small. Candidate is Tom Thumb (3x5, from the Adafruit GFX project); its licence is unverified. If no verifiable source exists, drop the 3x5 size instead of hand-crafting it.

Uncertain and to be checked: exact upstream file names and versions, licence text to record, and glyph coverage of each BDF against the charset in `glyphs.yaml`. Fetching from the network needs user approval.

## Milestones

- **M1**: Write `scripts/import-bdf-font.go` (`//go:build ignore`) converting a BDF into the `glyphs.yaml` matrices for one size. Verify: a unit test converts a small fixture BDF and compares rows.
- **M2**: Import the official 8x16, 6x12 and 7x13 fonts into `glyphs.yaml`. Also replace 3x5 with a verified upstream font, or remove the size. Keep 5x8 untouched. Verify: `git diff` on the 5x8 entries is empty.
- **M3**: Record source, version and licence per font (a `docs/` note plus a licence file if required), and update `docs/PixelFont5x8Glyphs.md` where it lists coverage gaps.
- **M4**: Regenerate the golden PNGs for the changed sizes. Verify: `make test-q1` passes and the 5x8 golden is byte-identical.

## Acceptance Criteria

- 3x5 (or its removal), 6x12, 7x13 and 8x16 matrices come from documented upstream fonts, with licences recorded.
- Every charset glyph exists in every size, or missing ones are listed explicitly with a fallback decision (upstream fonts may lack `✦`, `⚠`, `↳`, `┈`, `┄`).
- The default 5x8 glyphs are unchanged and remain the only hand-tuned font.
- The importer is re-runnable and tested.
