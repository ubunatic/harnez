# 460 — ANSI 256-color and 24-bit truecolor SGR support in readcard (428 follow-up)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Enhancement
**Related**: [428](428-code-review-follow-up-ansi-256-24-bit-color-extensions-and-telemetry-migration-test-coverage.md), [427](427-preserve-ansi-colors-in-stdin-render-path.md), `internal/readcard/ansi.go`

---

## Problem

`internal/readcard` parses only the 3/4-bit SGR colors plus reset/bold. Sequences such as
`\x1b[38;5;Nm`, `\x1b[48;5;Nm`, `\x1b[38;2;R;G;Bm` and `\x1b[48;2;R;G;Bm` are not handled
explicitly, so rich terminal output (diffs, styled tools) may lose colors or corrupt layout.

## /goal

Extended SGR sequences are either rendered (256-palette mapping, RGB passthrough) or safely
stripped, never dropping or shifting text in rendered PNG cards. Unit tests cover both forms
and malformed sequences.

## Notes

- Coordinate with 427 (stdin render path), which depends on the same parser.
- Re-verify against live code before starting.
