# 447 — Use B and hash as matrix color symbols

**Status**: Blocked — Dot8 experiment on hold; see 444
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: [444](444-fix-dot8-renderer-cell-pitch-for-mixed-glyphs.md), `internal/readcard/spec/glyphs-3x5.yaml`, `internal/readcard/glyph_spec.go`

---

## 1. Problem & Motivation

The editable glyph matrices currently use `1` and space for lit/unlit pixels. Prepare the format for future color channels by allowing `B` and `#` as matrix characters, with `B` meaning the default black/text channel. The naming must be established now without adding a general color feature.

## 2. /goal

Make `B` and `#` valid matrix symbols and make `B` the canonical symbol written by the serializer for the current default pixel color. On PNG cards, `B` must render using the existing theme-dependent default text color.

## 3. Scope and Constraints

- `B` is the default/black channel and maps to the current theme text color.
- `#` is accepted as an alternate matrix symbol for the same current default channel.
- Keep existing `1` matrices readable for compatibility, but normalize newly saved matrices to `B`.
- Do not add additional colors, palette configuration, or color-selection flags.
- Update YAML parsing, serialization, validation tests, and the 3×5 editable spec as needed.

## 4. Acceptance Criteria

- Matrices containing `B`, `#`, `1`, and spaces parse successfully.
- `B` and `#` produce identical current PNG pixels and use the theme-dependent default text color.
- Serialization writes `B` for default lit pixels and preserves spaces for unlit pixels.
- Existing glyph specs and tests remain compatible; add focused parser/serializer/render tests.

## On hold (2026-09-22)

Parked with the rest of the Dot8 chain: this glyph-matrix work only matters if Dot8
cards stay in play, and the token benchmark showed braille characters split under the
tokenizer badly enough that Dot8 must not be the primary input format (see
`docs/studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md`).
It also depends on 444's renderer-pitch fix landing first. Resume once 444 lands and a
clean 3x4 readability canary passes 3/3 on at least two agents.
