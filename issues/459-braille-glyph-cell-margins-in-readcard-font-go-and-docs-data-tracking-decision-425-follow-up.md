# 459 — Braille glyph cell margins in readcard `font.go` and docs/data tracking decision (425 follow-up)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [425](425-code-review-follow-up-telemetry-migration-logging-schema-version-drift-and-braille-glyph-spacing.md), [444](444-fix-dot8-renderer-cell-pitch-for-mixed-glyphs.md), `internal/readcard/font.go`

---

## Problem

`drawBrailleRune` maps dots to the bounding-box edges (`px := x + dotX[dot]*(width-1)`), so in
larger fonts (8x16) dots land in columns 0 and 7 with no horizontal margin. Adjacent Braille
runes visually merge. Separately, 425 left open whether the `docs/data/` benchmark logs are
tracked or gitignored.

## /goal

Braille runes keep consistent inter-glyph spacing and dot size across all font profiles, proven
by a test asserting margins in `read_test.go`. Decide and apply tracking or ignoring of
`docs/data/` session logs.

## Notes

- 444 covers the separate Dot8 renderer pitch; check that fix first, since it may change the
  shared assumptions.
- Re-verify against live code before starting.
