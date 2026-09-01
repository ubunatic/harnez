# 163 — Add named Unicode indicator sequences to the spec

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [155](155-braille-snake-timeout-indicator-spec.md), [157](157-spec-driven-usage-watch-chart-glyphs.md), [TUI design guidance](../docs/TUIDesign.md), `spec/indicators.yaml`, `spec/schemas/indicators.schema.json`

---

## 1. Problem & Motivation

`spec/indicators.yaml` currently makes each consumer declare literal glyph
frames. The commented Braille alternatives are useful discovery material, but
they are not reusable presets and make a user copy opaque Unicode strings into
every indicator definition.

Provide a small named sequence library in the indicator spec. Consumers such
as the time gauge must select a sequence by name rather than repeating its
frames inline. This makes the visual vocabulary discoverable, lets a user swap
styles with one readable setting, and keeps the renderer fully spec-driven.

## 2. Technical Specification

### Named library and references

- Extend `spec/indicators.yaml` with a top-level named sequence registry (for
  example `sequences:`). Each entry has a human-readable title and an ordered
  non-empty `frames` list.
- Replace literal `timeout-snake.frames` with a reference field such as
  `timeout-snake.sequence: braille-snake-2x3`. The loader resolves it to the
  declared frame list before rendering.
- Use a reference, not a second inline copy, for every indicator consumer that
  uses a multi-frame sequence. Single, semantically distinct bar endpoint
  glyphs may remain explicit until they are deliberately modeled as a
  sequence.
- Reject missing names, duplicate names, cycles (if aliases are supported),
  empty sequences, non-single-cell frames, and a consumer/sequence semantic
  mismatch with actionable errors. Do not add Go fallback frames.
- Preserve the existing time-gauge rule: its finite countdown consumes the
  resolved declared ordering verbatim and derives neither endpoint nor length
  from a hard-coded assumption.

### Initial presets

Ship concise, stable names for the researched one-cell vocabulary:

```yaml
sequences:
  block-deplete-8: { title: "Eighth-block countdown", frames: ["█", "▉", "▊", "▋", "▌", "▍", "▎", "▏", " "] }
  block-deplete-vertical-8: { title: "Vertical-block countdown", frames: ["█", "▇", "▆", "▅", "▄", "▃", "▂", "▁", " "] }
  shade-deplete-5: { title: "Shade countdown", frames: ["█", "▓", "▒", "░", " "] }
  quadrant-rotate-4: { title: "Quadrant spinner", frames: ["▘", "▝", "▗", "▖"] }
  half-block-rotate-4: { title: "Half-block spinner", frames: ["▄", "▌", "▀", "▐"] }
  box-line-rotate-4: { title: "Box-line spinner", frames: ["╷", "╴", "╵", "╶"] }
  braille-orbit-8: { title: "Braille orbit", frames: ["⡀", "⠄", "⠂", "⠁", "⠈", "⠐", "⠠", "⢀"] }
  braille-classic-10: { title: "Classic Braille spinner", frames: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"] }
  braille-snake-2x3: { title: "2×3 Braille timeout snake", frames: ["⠿", "⠷", "⠧", "⠇", "⠃", "⠁", "⠀"] }
```

The first three are determinate depletion styles; the rotating/orbiting styles
are indeterminate and must not silently be used as a timeout countdown.
Retain `braille-snake-2x3` as the default time-gauge selection. Emoji clocks,
moon phases, and sextant blocks are intentionally excluded from the default
library because fixed-cell width or font coverage is unreliable.

### Validation and geometry

- Validate Unicode display width for fixed-slot frames, rather than byte
  length. A frame must occupy one terminal cell in supported rendering.
- A literal space is permitted as an erase/end frame. U+2800 (`⠀`) remains
  valid where explicitly chosen as part of a Braille sequence.
- Keep ANSI color configuration separate from sequence geometry; absent/null/
  empty backgrounds must still yield valid foreground-only rendering.

## 3. Implementation & Verification Plan

- [ ] Define the registry/reference schema and migrate `spec/indicators.yaml`
  from commented alternatives to named presets.
- [ ] Update the strict embedded loader and resolved indicator options without
  leaking spec loading into `internal/rograph`.
- [ ] Add table-driven tests for each preset's exact ordering, name resolution,
  unknown/duplicate/invalid sequence errors, variable valid lengths, and the
  time gauge's finite countdown behavior.
- [ ] Test one-visible-cell geometry and ANSI-stripped alignment for every
  selected gauge/bar/spark output; include literal-space and U+2800 endpoints.
- [ ] Run focused usage/spec tests, `go test ./...`, `make install`, and
  `harnez status`.
