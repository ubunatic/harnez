# 432 — Consolidate pixel font specs: merge 5x8 extensions, drop dead profile, one spec per font

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactoring

---

## Context

Follow-up to 429 (glyphs to YAML spec). `font_5x8.yaml` is the hand-tuned authority for `Font5x8`, but its glyphs are spread across several spec files in `internal/readcard/spec/`:

- `font_extensions.yaml` (`retro_pixel_5x8`, 18 glyphs) holds box drawing, arrows, `✓ ✗ … ↳ •` in a different row format (8-bit binary strings, 2 bits clipped by the 6-wide cell) than `font_5x8.yaml` (6-char `1`/space strings).
- `glyphs.yaml` `profiles.default` is dead: `DrawRune` checks `Font5x8.Glyphs` first, so `glyphPatternForFont`'s `Font5x8` case is never reached for any glyph the font defines.
- `glyphs.yaml` mixes the shared charset, base 5x7 fallback patterns (used by every font except `Font5x8`), and per-size profiles.
- The `⣿` entry in `font_5x8.yaml` is never used for drawing; `DrawRune` routes all `U+2800-28FF` to `drawBrailleRune`.

Other fonts cannot simply derive from the 5x8 glyphs: scaling is lossy and `glyphs.yaml` explicitly keeps size-specific matrices.

## Scope

- **M1**: Merge the `retro_pixel_5x8` extension glyphs into `font_5x8.yaml` in its string-row format; remove the binary-row loader use for that font. Verify: `TestGoldenFontAssets` for 5x8 unchanged.
- **M2**: Remove `profiles.default` and the `Font5x8` branch in `glyphPatternForFont`. Verify: full readcard tests plus golden assets unchanged.
- **M3**: Decide on the `⣿` entry: remove it, or document it as reference only. Verify: golden 5x8 unchanged.
- **M4** (larger): One spec file per font (`font_3x5.yaml`, `font_6x12.yaml`, ...), with `glyphs.yaml` reduced to the shared unicode fallback; move `charset` to its own file. Verify: all five golden PNGs byte-identical.

## Acceptance Criteria

- `font_5x8.yaml` alone defines every glyph rendered by `Font5x8` (except procedural Braille).
- No dead spec entries or dead branches remain; `spec/schemas/` updated to match.
- All golden PNGs in `docs/data/` are unchanged and `make test-q1` passes.
- `docs/PixelFont5x8Glyphs.md` reflects the new file layout.
