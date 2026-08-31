# 132 — Spec-driven btop-style superscript hotkeys and single hidden-count hint for `harnez usage --watch`

**Status**: Closed — resolved in `b23ae32`, `aee953a`
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[094-usage-watch-controls-overlay-and-presets]] (built the `?` controls overlay + presets but explicitly kept the old per-key badges and direct toggles as "secondary controls" — this ticket revisits that display layer), [[093-usage-tui-layout-planner]], [[049-running-agent-processes-watch-panel]], [[050-remote-host-flag-and-watch-hotkey]], `docs/Spec.md` (spec-driven architecture convention), `internal/usage/watch.go`, [[131-watch-debug-freshness-countdown-overlay]] (sibling label-rendering ticket — see its Scope Boundary note; implement this ticket first)

## Sequencing Note

Implement before 131. 131's `!` debug overlay depends on this ticket's
`spec/actions.yaml` key registry existing (so `!` gets added there rather
than as another hardcoded `switch` case), and 131 touches per-agent row
labels while this ticket touches box titles — different render targets, so
there is no acceptance-criteria conflict between the two, only an ordering
dependency.

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

## Resolution

Implemented directly in `spec/actions.yaml` + `spec/schemas/actions.schema.json`
(new `spec/` directory, first use in this project) and
`internal/usage/actionsspec.go` + `internal/usage/watch.go`:

- `spec/actions.yaml` is the single source of truth for every `--watch`
  hotkey: key(s), action name, display symbol, category (`box`/`data`/
  `mode`/`session`), and — for box-toggle actions — the `watchSections`
  field it controls. Embedded via `//go:embed` (`embed.go`) and parsed/
  validated at load time by `internal/usage/actionsspec.go`
  (`parseWatchActionsYAML`), which returns a clear error rather than
  panicking on malformed or schema-violating input (unit-tested in
  `actionsspec_test.go`).
- `watch.go`'s `dispatchWatchKey`/`applyWatchSectionKey` now look up each
  keypress's action name via the spec-backed `mustWatchActions()` and only
  keep the toggle/dispatch *logic* in Go, per `docs/Spec.md`. Box titles
  (`buildAgentBox`, `buildHistoryBox`, `buildProcessesBox`, `buildLoadBox`,
  `buildAllUsageBox`) render a superscript digit via `watchBoxSymbol`
  instead of a bracketed `[X]` badge.
- **Key collision decision**: the old mixed letter+digit scheme
  (`C`/`G`/`O`/`1`/`2`/`3`, `H`/`4`, `T`/`5`, `P`/`6`, `L`/`7`) is replaced by
  one numbered scheme: `1` All Usage, `2` Claude, `3` AGY, `4` Codex,
  `5` History, `6` Processes, `7` Load. Digit meanings shift for every box
  except this ticket kept none of the old digit assignments stable — this
  was deliberate: box numbering now follows the boxes' on-screen order
  (All Usage first, then agents, then History/Processes/Load) rather than
  preserving arbitrary legacy digits. The `C`/`G`/`O`/`H`/`P`/`L` letter
  aliases are dropped entirely (superseded by the numbered scheme, per the
  ticket's "consolidate mixed letter+digit toggles" scope item). `[a]` is
  kept working as a documented compat alias for All Usage (acceptance
  criteria explicitly required `a`/`A` to keep working), alongside `[A]`
  reset, `[T]` token-row toggle (letter-only — it's a data row, not a box,
  so it never got a digit), `[m]` preset cycle, `[r]` remote, `[q]` quit,
  and `[?]` controls, all unchanged. The overlay's own dismiss-only Enter
  key stays a hardcoded local behavior, not spec'd (it has no visible
  symbol anywhere to source from spec).
- The header's per-hidden-box badge list (`hidden: [H] [P] ...`) is
  replaced by a single `N hidden (press ? for controls)` count. The
  separate "… hidden — terminal too short" height-overflow drop note
  (a different mechanism — boxes present but not fitting vertically) is
  unchanged, per the ticket's scope.

Verified: `go build ./...`, `go vet ./...`, `go test -race ./internal/usage/...`
(all pass, including new `actionsspec_test.go` spec-loading/validation/
dispatch coverage and `TestBuildWatchFrame_HiddenCountAtZeroOneAndN`), and a
manual `harnez usage --summary` run confirming the superscript titles and
hidden-count hint render correctly against the real binary.
