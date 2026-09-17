# 408 — Add soft-wrapping and ellipsis to harnez read image rendering to eliminate line truncation

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Feature / Multimodal Context & Readcard
**Related**: #406, #407, #395, #399

---

## 1. Problem Statement & Motivation

When files with long lines (e.g. diffs, verbose code statements, long URLs, JSON, prose markdown) are rendered to visual PNG cards with `harnez read -I`:
1. **Single-Column Width Underutilization**: `maxLineLen` was artificially clamped to `120` characters, restricting `cardWidth` to ~804px even though a standard 1568px canvas has capacity for ~240 characters. Any line longer than 120 chars was cut off.
2. **Hard Truncation Without Wrapping or Indicators**: When a line exceeds column width (either in multi-column mode or on ultra-long lines in 1-column mode), token rasterization currently halts abruptly at the column boundary, silently dropping text without indicating truncation.

To ensure **zero information loss** across visual context cards, `harnez read` needs:
- Dynamic single-column expansion up to `MaxDimension` (~240 chars).
- Automatic soft-wrapping with continuation markers (`↳ ` or indented wrapped lines) for overflow lines.
- Visual ellipsis (`…`) when truncation occurs or when `--wrap=truncate` is specified.

---

## 2. Technical Scope & Implementation Plan

### 2.1 Expand 1-Column Canvas Width
- In `RenderFileToCards` (`internal/readcard/render.go`):
  - In 1-column mode, allow `maxLineLen` to grow up to the available capacity of `opts.MaxDimension` (e.g. `(MaxDimension - paddingX*2 - gutterWidth - 16) / cw`).
  - Calculate `cardWidth` dynamically based on the actual longest line up to `opts.MaxDimension`.

### 2.2 Soft-Wrapping Engine (`WrapLines`)
- When lines exceed the available column character width `colCharWidth = (colWidth - gutterWidth - 16) / cw`:
  - Split long lines into a main line and one or more continuation lines.
  - Continuation lines are rendered on subsequent rows with:
    - An empty/blank line-number gutter.
    - A continuation glyph or indent (`↳ ` / `  `).
    - Syntax highlighting preserved across wrapped chunks.
- Update `totalLines` / page height calculations to account for wrapped continuation rows.

### 2.3 Visual Ellipsis Marker
- When text reaches the absolute column boundary without soft-wrapping (or at the end of maximum wrapped continuation lines), render a clear `…` indicator in `theme.Comment` or `theme.Text` color.

### 2.4 CLI Flag
- Support `--wrap=soft|truncate` (default: `soft` for visual cards to guarantee zero loss).

---

## 3. Acceptance Criteria

- [ ] 1-column mode expands `cardWidth` dynamically to fit lines up to `MaxDimension` (up to ~240 characters).
- [ ] Overflow lines in multi-column mode or long lines soft-wrap onto continuation rows with a continuation marker and blank line-number gutter.
- [ ] Truncated or boundary lines render a clean `…` ellipsis rather than cutting off invisibly.
- [ ] Unit tests in `internal/readcard/read_test.go` and `cmd/harnez/read_test.go` verifying soft-wrapping, continuation rows, and width expansion.
- [ ] Verify `make test-q1`.
