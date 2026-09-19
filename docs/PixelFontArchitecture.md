# Pixel Font Architecture and Lessons

How `internal/readcard` gets its bitmap glyphs, why it looks the way it does, and what went wrong on the
way. Per-glyph design of the default font lives in [PixelFont5x8Glyphs.md](PixelFont5x8Glyphs.md).
Tracking tickets: 429, 430, 431 (spec move, `%`, review), 432/433 (consolidation, upstream fonts), 434 (per-size specs).

## Current shape

- `MonospaceFont{Name, CharWidth, CharHeight, Ascent, Glyphs map[rune][]byte}`; one byte per row, MSB first.
- Sizes and cells: `3x5`→4x6, `5x8`→6x8 (default, `Font5x8`), `6x12`→7x12, `7x13`→7x13, `8x16`→8x16.
- Braille (U+2800-28FF) is drawn procedurally in `drawBrailleRune`, never from a table.
- `internal/readcard/spec/charset.yaml` holds the ordered charset; `glyphs-<size>.yaml` holds char → rows (`1`/space) per size.
  `glyphs-5x8.yaml` is complete and hand-tuned. The other files list only glyphs the upstream BDF lacks
  (`TestGlyphSpecsHaveNoUpstreamOverlap`); the BDF wins on conflict.
- Upstream fonts (Tom Thumb 3x5, Spleen 6x12/8x16, X11 misc-fixed 7x13) are embedded from
  `internal/readcard/upstream/*.bdf`; provenance and licences in `third_party/fonts/README.md`.
  `go:embed` cannot reach `third_party/`, hence the copies next to the package.
- Non-default fonts load lazily (`sync.Once` in `DrawRune`) so `harnez --version` stays near 28 ms.

## Verification

- Golden PNGs `docs/data/golden-font-*.png` are compared byte-for-byte (`TestGoldenFontAssets`).
  `go run ./scripts/generate-golden-fonts.go` regenerates them and checks 5x8 against the hand-made
  `golden-font-5x8-import.png`.
- Property tests beat pinned shapes: 5x8 gets exact row counts for `%`; other sizes only need ink in all four quadrants.

## Lessons

1. **One list beats layered specs.** Four YAMLs, scaled patterns and profiles hid dead entries and precedence bugs
   in `DrawRune` (unused default profile, an unreachable `⣿` entry). A flat char → size → rows map made them obvious.
2. **Derived fonts leak.** 6x12 was derived from the 5x8 table, so 5x8 extensions changed 6x12 goldens. Do not derive one size from another.
3. **Overwrite-only importers leave stale data.** When upstream lacks a glyph the old hand-crafted entry survives (3x5, 8x16 box
   drawing and `⚠ ✦ ✓ ✗`). Spec only what upstream lacks, and test for overlap (434 M2, done).
4. **BDF placement needs `FONTBOUNDINGBOX`.** Ascent is height + y offset, plus x offset; ignoring it shifted glyphs vertically.
5. **Edit YAML through `yaml.Node`, never re-marshal.** The first importer rewrote the file (17k-line diff, comments lost).
6. **Anything parsed at package init is startup cost.** Importing every BDF glyph made a 1.3 MB spec and `--version` went 21 → 130 ms.
   Restrict to the charset and load lazily.
7. **Append new charset chars at the end.** The import-PNG comparison is positional; inserting mid-string broke it.
8. **Do not pin upstream shapes in tests.** They change with the font; check properties instead.
9. **Small-font design is about the nearest confusable glyph**, not fidelity (see the 5x8 doc). Rounded box corners equal square ones at 5 px.

## Known gaps

- Upstream lacks `→ ← ✓ ✗` (6x12) and `✓ ✗` (7x13); see `third_party/fonts/README.md`.
- Fallback for glyphs missing everywhere is still `?`; 434 M3 recommends the unscaled 5x8 glyph.
