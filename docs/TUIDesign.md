# TUI Design — Single-Cell Time Indicators

Consult this when adding or changing a timeout gauge, progress/usage bar,
sparkline, spinner, or other repeatedly redrawn terminal indicator.

## Choose the semantic before the glyph

- **Determinate elapsed/countdown state** needs an ordered fill/depletion
  sequence. Do not use a cyclic spinner when the viewer should be able to see
  time remaining.
- **Indeterminate work** can use a rotating marker or orbiting dot.
- **Attention/pulse** can fade or breathe, but does not communicate progress.

The configured sequence is the contract: do not infer its length, its first or
last frame, or a required full/empty glyph. This permits compact patterns such
as a 2x3 Braille snake as well as 8-step block depletion.

## Safe one-cell presets

```yaml
# Determinate countdowns (full to empty)
block-deplete-8: ["█", "▉", "▊", "▋", "▌", "▍", "▎", "▏", " "]
block-deplete-vertical-8: ["█", "▇", "▆", "▅", "▄", "▃", "▂", "▁", " "]
shade-deplete-5: ["█", "▓", "▒", "░", " "]

# Indeterminate work
quadrant-rotate-4: ["▘", "▝", "▗", "▖"]
half-block-rotate-4: ["▄", "▌", "▀", "▐"]
box-line-rotate-4: ["╷", "╴", "╵", "╶"]
braille-orbit-8: ["⡀", "⠄", "⠂", "⠁", "⠈", "⠐", "⠠", "⢀"]
braille-classic-10: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]

# Pulse / attention, not a measure of progress
shade-pulse-5: ["█", "▓", "▒", "░", " "]
dot-pulse-5: ["·", "•", "●", "•", "·"]
```

Reverse a determinate sequence for growth. A Braille snake is also a valid
countdown when its explicit frames clear dots in a meaningful path; it need
not start at `⣿` or finish at a particular blank glyph.

## Terminal constraints

- Every frame in a fixed gauge slot must occupy **one terminal column**. Test
  display width, not byte length or rune count, and test the whole styled
  output after stripping ANSI escapes.
- Avoid emoji clocks (`🕛…🕚`) and moon phases (`🌑…🌘`) in fixed-width layouts:
  their presentation and width vary by terminal and font, often to two cells.
- U+2800 (`⠀`, Braille blank) is a real Braille pattern, not a space. Prefer a
  literal space as the erase frame when padding/erase semantics allow it;
  retain U+2800 when the declared Braille sequence deliberately requires it.
- Sextant block mosaics (U+1FB00–U+1FB3B) offer attractive 2x3 patterns but
  have uneven font coverage. Treat them as opt-in rather than defaults.
- ANSI colors are styling around a frame, never part of its geometry. Apply
  style after padding/positioning; permit foreground-only rendering when no
  background SGR is configured.

## References

- [Unicode Block Elements](https://www.unicode.org/charts/nameslist/n_2580.html)
  defines the eighth-block increments used by determinate bars.
- [cli-spinners](https://github.com/sindresorhus/cli-spinners) documents the
  classic Braille spinner and common terminal animation conventions.
- [Unicode Braille patterns](https://www.unicode.org/versions/Unicode17.0.0/core-spec/chapter-21/)
  defines the 2x4 dot layout and all 256 patterns.
- [Unicode Sextant Blocks](https://www.unicode.org/charts/nameslist/n_1FB00.html)
  documents the 2x3 mosaics and their glyph-coverage caveat.
