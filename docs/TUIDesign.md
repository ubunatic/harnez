# TUI Design — Indicators, Panels & Keybinding Invariants

Consult this when adding or changing a timeout gauge, progress/usage bar,
sparkline, spinner, multi-panel grid layout, or raw-mode hotkey dispatcher.

## Choose the semantic before the glyph

- **Determinate elapsed/countdown state** needs an ordered fill/depletion
  sequence. Do not use a cyclic spinner when the viewer should be able to see
  time remaining.
- **Indeterminate work** can use a rotating marker or orbiting dot.
- **Attention/pulse** can fade or breathe, but does not communicate progress.

The configured sequence is the contract: do not infer its length, its first or
last frame, or a required full/empty glyph. This permits compact patterns such
as a 2x3 Braille snake as well as 8-step block depletion.

Named sequences live once in `spec/indicators.yaml`; consumers select them by
`sequence` name and never repeat frame lists inline. Each registry entry has a
semantic kind. Resolve and validate that kind in the application layer before
passing plain glyph options to `internal/rograph`. Sequence aliases are not
part of the model.

## Safe one-cell presets

The canonical titles and exact frames are in the spec registry:

- Countdown: `block-deplete-8`, `block-deplete-vertical-8`,
  `shade-deplete-5`, `braille-snake-2x3`.
- Indeterminate: `quadrant-rotate-4`, `half-block-rotate-4`,
  `box-line-rotate-4`, `braille-orbit-8`, `braille-classic-10`.
- Renderer scales: `horizontal-eighths-7` for partial bars and
  `vertical-block-scale-8` for sparklines.

Reverse a determinate sequence for growth. A Braille snake is also a valid
countdown when its explicit frames clear dots in a meaningful path; it need
not start at `⣿` or finish at a particular blank glyph.

## Terminal constraints

- Every frame in a fixed gauge slot must be one Unicode rune occupying **one
  terminal column**. Validate display width in addition to rune count; byte
  length is irrelevant. Test the whole styled output after stripping ANSI
  escapes (`runewidth.StringWidth(stripANSI(line))`).
- Format dynamic labels and numeric/percentage fields with a fixed column
  budget (e.g. `%3.0f%%` for 0–100%, or sized duration fields) so transitioning
  from `99%` to `100%` never shifts or jitters adjacent visual columns.
- For 2-samples-per-cell Braille sparklines, history series must be padded to
  `2 * TargetWidth`, not `TargetWidth`. If a remote host or offline feed returns
  fewer samples than the visual budget, pad with baseline zero glyphs so remote
  and local charts maintain identical cell widths.
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

## Panel visibility and keybinding invariants

- **Every rendered panel must be backed by the view model**: Never append or render
  a box directly based on options or configuration alone (e.g. `opt.RemoteLoadHost != ""`).
  Route all panel drawing through explicit boolean fields in the visibility state
  (`watchSections.RemoteLoad`). Unconditional rendering bypasses user layout presets
  (`compact`, `agents-only`), breaks hidden-panel counts, and defeats reset-to-default controls.
- **Visual hotkey affordance contract**: If a panel title or footer displays a bracketed
  hotkey badge (e.g. `[R]` in `[R] Remote Load` or `[r]emote` in the footer), pressing
  that hotkey must never be a silent no-op.
- **Multi-source fallback dispatch**: When a single hotkey has distinct meanings depending
  on runtime context (e.g. `r` switching primary `--host` when configured vs. toggling the
  auxiliary `load.watch_host` panel in local sessions), the key dispatcher must fall back
  gracefully across all active configurations rather than silently dropping unhandled paths.

## References

- [Unicode Block Elements](https://www.unicode.org/charts/nameslist/n_2580.html)
  defines the eighth-block increments used by determinate bars.
- [cli-spinners](https://github.com/sindresorhus/cli-spinners) documents the
  classic Braille spinner and common terminal animation conventions.
- [Unicode Braille patterns](https://www.unicode.org/versions/Unicode17.0.0/core-spec/chapter-21/)
  defines the 2x4 dot layout and all 256 patterns.
- [Unicode Sextant Blocks](https://www.unicode.org/charts/nameslist/n_1FB00.html)
  documents the 2x3 mosaics and their glyph-coverage caveat.
