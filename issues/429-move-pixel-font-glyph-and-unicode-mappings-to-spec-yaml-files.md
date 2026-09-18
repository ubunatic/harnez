# 429 — Move pixel font glyph and unicode mappings to spec/ YAML files

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactoring

---

## Context

Per the project spec system convention (`@docs/other/Spec.md`: *"YAML spec files as single source of truth; Go code must not duplicate spec values"*), bitmap and unicode glyph mappings currently hardcoded in Go code (such as `internal/readcard/unicode.go` and `font.go`) should be moved to declaratively defined `spec/` YAML files or embedded YAML assets and loaded dynamically or code-generated at build time.

Currently:
- `unicodeGlyphs` in `internal/readcard/unicode.go` defines pixel matrices (5x7 grids) directly in a Go `map[rune][]string`.
- Additional custom glyphs and degree/symbol rasterizations are in Go logic rather than declarative spec assets.

## Proposed Actions

- [ ] Create a declarative YAML spec (e.g. `spec/glyphs.yaml` or `internal/readcard/spec/glyphs.yaml`) defining custom glyphs, grid sizes, and rune mappings.
- [ ] Update `internal/readcard` to load/embed or generate glyph definitions from the YAML spec.
- [ ] Update `internal/readcard/font_unicode_test.go` and `read_test.go` to validate against the YAML spec definitions.
- [ ] Verify test suite passes with `make test-q1`.

## Acceptance Criteria

- Glyph pixel matrices and unicode mappings are defined in YAML spec files without manual duplication in Go maps.
- Glyph rendering and rasterization behavior remains identical and passes all tests.
- Single-test Quota-1 test pass (`make test-q1`).
