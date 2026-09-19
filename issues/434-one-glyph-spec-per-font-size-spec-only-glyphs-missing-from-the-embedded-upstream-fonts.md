# 434 — One glyph spec per font size; spec only glyphs missing from the embedded upstream fonts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactoring

---

## Context

Follow-up to 432 and 433. Today `internal/readcard/spec/glyphs.yaml` is one file holding a matrix per glyph and per size. 433 copied upstream glyphs (Tom Thumb 3x5, Spleen 6x12/8x16, X11 misc-fixed 7x13) into it through `scripts/import-bdf-font.go`, filtered to the charset. Problems:

- One large file is hard to edit; a size's glyphs are scattered across it.
- Upstream data is duplicated in the spec, although the BDFs already live in `third_party/fonts/`.
- The importer only overwrites glyphs it finds, so glyphs *missing* upstream keep their old hand-crafted or derived entries. 3x5 and 8x16 still carry such leftovers (box drawing, `⚠`, `✦`, `✓ ✗`). Verified: none of the four BDFs has `✓ ✗ ⚠ ✦`, Spleen 6x12 also lacks `→ ←`, and Tom Thumb has no box drawing.

Direction: embed the upstream BDFs and read them directly, so their full coverage is used without listing glyphs. Each size gets its own YAML spec that lists **only** glyphs the upstream font lacks. The default 5x8 has no upstream font, so its spec stays complete.

## Design

- `spec/glyphs-<size>.yaml` per size (`3x5`, `5x8`, `6x12`, `7x13`, `8x16`); the shared charset moves to its own small file or stays in one place.
- Non-default sizes: BDF is the primary source; the size's YAML supplies fill-ins only.
- A test fails if a size's YAML lists a glyph that the upstream BDF already has.
- Upstream BDFs are parsed lazily, only when that size is first used, so `harnez --version` stays near the current ~28 ms (it regressed to ~130 ms when the spec grew to 1.3 MB).

## Uncertainties to resolve while implementing

- **Fallback for glyphs in neither source:** `?` (today), or the default 5x8 glyph drawn unscaled in the cell. Recommendation: the 5x8 glyph, so status marks stay readable. Decide and record.
- **Embedding:** `go:embed` cannot reach `third_party/` from `internal/readcard`. Either move the BDFs into the package tree or add a small embed package next to them.
- **Parse cost:** `7x13.bdf` is 400 KB (3,226 glyphs). Measure lazy parsing; if too slow, pre-generate a compact form at build time.
- Whether the missing glyphs (`✓ ✗ ⚠ ✦`, box drawing for 3x5, `→ ←` for 6x12) need hand-drawn fill-ins, given the earlier decision to hand-craft nothing outside the default font. That may mean accepting the fallback instead.

## Milestones

- **M1** (done in 363d2c9, lazy loading of the spec font pending commit): Embed the BDFs and add a BDF reader in `internal/readcard` (reuse the baseline and bounding-box logic from `scripts/import-bdf-font.go`), loaded lazily per size. Verify: unit tests on a fixture BDF; `harnez --version` timing unchanged.
- **M2**: Split the spec into one YAML file per size and remove upstream copies and leftover derived entries from the non-default files, keeping only glyphs the BDF lacks. Update `spec/schemas/`. Verify: a test asserts no overlap between a size's YAML and its BDF.
- **M3**: Implement the chosen fallback for glyphs missing everywhere. Verify: tests for a glyph present only in YAML and one present in neither.
- **M4**: Remove `scripts/import-bdf-font.go` and its test, update `third_party/fonts/README.md` and `docs/PixelFont5x8Glyphs.md`, regenerate the goldens. Verify: `make test-q1`; the 5x8 golden stays byte-identical.

## Acceptance Criteria

- Each size has its own spec file; non-default files contain only glyphs absent from the upstream BDF.
- Rendering uses the full upstream coverage for 3x5, 6x12, 7x13 and 8x16.
- No duplicated upstream data or leftover hand-crafted entries remain in non-default specs.
- Startup time is not measurably worse than today (about 28 ms for `harnez --version`).
- The 5x8 glyphs and golden are unchanged.
