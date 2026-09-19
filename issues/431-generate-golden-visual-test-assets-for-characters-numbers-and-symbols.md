# 431 — Generate golden visual test assets for characters, numbers, and symbols

**Status**: Closed — implemented in readcard spec YAML, 5x8 font tuning, golden assets
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Testing

---

## Context

To ensure bitmap font quality, prevent visual regressions, and benchmark OCR / multimodal token consumption across LLMs, we need a canonical set of "golden" visual data assets stored under `docs/data/` (or test fixtures).

These golden images should systematically render:
- Numbers `0-9`
- Lowercase letters `a-z`
- Uppercase letters `A-Z`
- Punctuation and symbols (`!@#$%^&*()_+-=[]{}|;':",.<>/?` etc.)
- Common Unicode glyphs, box-drawing chars, and Braille cells across all supported font sizes (3x5, 5x8, 6x12, 7x13, 8x16).

## Proposed Actions

- [ ] Create a generation script/test utility (e.g. in `internal/readcard/read_test.go` or `scripts/generate-golden-fonts.go`) to render full charset matrices to PNG.
- [ ] Save canonical golden PNG assets to `docs/data/` (e.g. `docs/data/golden-chars-*.png`).
- [ ] Add regression tests that compare rendered output against expected golden hashes or pixel buffers.
- [ ] Verify test suite passes via `make test-q1`.

## Acceptance Criteria

- Standard golden PNG assets covering 0-9, a-z, A-Z, symbols, and Unicode glyphs exist in `docs/data/`.
- Automated tests or generators exist to verify fidelity against golden definitions.
- All tests pass under Quota-1 rules.
