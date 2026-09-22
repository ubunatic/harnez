# 459 — Braille glyph cell margins in readcard `font.go` and docs/data tracking decision (425 follow-up)

**Status**: Blocked — Dot8 experiment on hold; see 444
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

## On hold (2026-09-22)

Parked with the rest of the Dot8 chain. The larger-dot canary's M1 results (which
motivated tuning Braille dot geometry like this) were invalidated by fixture and
environment contamination; the corrected re-run shows 5x7 is not a win (-5% area,
+19% bytes) and only 3x4 saves meaningfully (-66% area, -45% bytes), at a dot size no
agent has been shown to read (see `docs/studies/2026-09-20-dot8-larger-dot-card-canary.md`).
Fixing margins on a geometry that may not survive is premature. Resume once 444 lands
and a clean 3x4 readability canary passes 3/3 on at least two agents.
