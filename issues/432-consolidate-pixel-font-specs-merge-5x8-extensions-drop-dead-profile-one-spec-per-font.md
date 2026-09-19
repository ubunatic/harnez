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

## Outcome

Done via a simpler design than proposed: `internal/readcard/spec/glyphs.yaml` is now the single list of every glyph with one matrix per font size. `font_5x8.yaml`, `font_extensions.yaml`, `font_tables.yaml`, the scaled 5x7 fallback patterns, profiles and `unicode.go` are gone. Golden PNGs are byte-identical.

## Remaining

- Fill the remaining coverage gaps (currently render as `?`): 6x12 lacks `\`, `` ` ``, `~` and `┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼ → ← ✓ ✗`; 7x13 lacks the 13 box/arrow symbols. 5x8 is complete. Regenerate the goldens afterwards.
