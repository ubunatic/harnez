# 132 — Spec-driven btop-style superscript hotkeys and single hidden-count hint for `harnez usage --watch`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[094-usage-watch-controls-overlay-and-presets]] (built the `?` controls overlay + presets but explicitly kept the old per-key badges and direct toggles as "secondary controls" — this ticket revisits that display layer), [[093-usage-tui-layout-planner]], [[049-running-agent-processes-watch-panel]], [[050-remote-host-flag-and-watch-hotkey]], `docs/Spec.md` (spec-driven architecture convention), `internal/usage/watch.go`

## Problem

Issue 094 added a `?` controls overlay and mode presets to reduce hotkey
chaos in `harnez usage --watch`, but left the underlying key set and its
in-TUI representation unchanged: direct toggles are still `C`/`G`/`O`/
`1`/`2`/`3`, `H`/`4`, `T`/`5`, `P`/`6`, `L`/`7`, `a`/`A`, `r`, `q` — a mix of
letters and digits with no consistent visual language, and hidden-box state
is still communicated via a badge per hidden box in the header/title area.
With more panels landing over time (issue 131 adds another overlay concept
on top), the key surface keeps growing without a systematic mapping.

Additionally, the key→action mapping lives only as Go `switch` cases in
`watch.go` (e.g. `watch.go:476-529`) — there's no single source of truth a
person or another tool can read to see "what does every key currently do,"
which is exactly the kind of hardcoded-in-code table `docs/Spec.md`'s
spec-driven convention exists to prevent duplicating.

## btop Comparison (carried over from issue 094's research)

btop maps numeric keys `1`-`4`(+) directly to box visibility and renders
compact superscript-style numeric glyphs next to box titles to show which
number toggles which box, rather than spelling out `[1]`/`[2]` badges for
every box. This is denser and reads as a coherent numbered system rather
than an ad hoc mnemonic letter soup.

## Scope

1. **Superscript numeric toggles**: Replace/consolidate the current mixed
   letter+digit direct toggles with a btop-style numbered scheme — digits
   `1`-`5` (or as many as there are toggleable top-level boxes) each map to
   one box, rendered next to that box's title/label as a superscript digit
   (`¹`²³⁴⁵`) rather than a bracketed `[1]` badge. This is a pure key
   remap + rendering change; it does not add new toggleable panels.
2. **Single hidden-count hint, not per-box badges**: Replace the current
   per-hidden-box badge display with one compact summary, e.g. `3 hidden`
   or similar, plus the existing `?` hint to open the full controls
   overlay for details. The controls overlay (issue 094) remains the place
   to see which specific boxes are hidden and why — the header/footer no
   longer needs to enumerate them individually.
3. **Spec-driven hotkey registry**: Introduce `spec/actions.yaml` (first
   use of the `spec/` convention in this project — bootstrap the directory
   per `docs/Spec.md`'s standard layout, `spec/schemas/actions.schema.json`
   included) defining, for every watch hotkey: the key(s) that trigger it,
   the action name, and the display symbol (e.g. the superscript glyph for
   numbered toggles, or the literal key for letter-based actions like `q`/
   `r`/`?`/`m`). Embed it via `//go:embed` per the spec convention. Replace
   the hardcoded `switch` key-to-symbol mapping in `watch.go` with a lookup
   against this spec, so the spec is the single source of truth for "what
   key does what and how is it shown" — Go code must not duplicate the
   mapping.
4. Keep this scoped to `harnez usage --watch`'s own hotkeys; do not attempt
   to spec-ify unrelated CLI flags or other commands' key handling.

## Acceptance Criteria

- [ ] `spec/actions.yaml` + `spec/schemas/actions.schema.json` exist,
      documenting every `harnez usage --watch` hotkey: key(s), action,
      and display symbol.
- [ ] `watch.go`'s key dispatch and title/hint rendering read from the
      embedded spec rather than a hardcoded Go switch-to-symbol map (the
      dispatch *logic* can stay Go — only the key/symbol *mapping* moves
      to spec).
- [ ] Digit keys `1`-`N` toggle their respective top-level boxes and are
      rendered as superscript digits next to each box's title.
- [ ] The hidden-box indicator is a single count/summary, not one badge
      per hidden box; `?` still opens the full controls overlay (issue 094)
      for the detailed breakdown.
- [ ] Existing non-numbered hotkeys (`q`, `r`, `?`, `m`, `a`/`A`) keep
      working; document any keys that had to change to avoid collisions
      with the new numbered scheme.
- [ ] Unit tests cover: spec loading/validation, key dispatch against the
      spec-driven map, and the hidden-count rendering at 0/1/N hidden boxes.
- [ ] `go test -race ./internal/usage/...` passes clean.
- [ ] `harnez status` confirms tracker sync after filing/closing.
