# 430 — Fix percent (%) symbol glyph rendering in bitmap fonts

**Status**: Closed — implemented in readcard spec YAML, 5x8 font tuning, golden assets
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug

---

## Context

In generated readcard visual outputs (such as `docs/data/harnez-read-chars-001.png`), the percent sign (`%`) glyph bitmap appears visually distorted or off-balance across bitmap font variants (e.g. 5x8, 6x12, 7x13, 8x16).

The bitmap matrix for `%` in `internal/readcard/font.go` / font tables needs correction so that the upper and lower loops and the diagonal slash align cleanly at pixel boundaries.

## Proposed Actions

- [ ] Inspect the `%` bitmap matrix across font sizes (`Font3x5`, `Font5x8`, `Font6x12`, `Font7x13`, `Font8x16`).
- [ ] Adjust the pixel pattern for `%` to ensure crisp visual alignment and legibility.
- [ ] Add visual/matrix regression tests in `internal/readcard/read_test.go`.
- [ ] Verify test suite passes via `make test-q1`.

## Acceptance Criteria

- The `%` character renders cleanly without pixel artifacts or distorted diagonal/circle alignment.
- All tests pass under Quota-1 rules.
