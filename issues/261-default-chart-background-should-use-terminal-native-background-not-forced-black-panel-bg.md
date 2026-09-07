# 261 — Default chart background should use terminal-native "background", not forced black panel-bg

**Status**: Closed — Default chart background now uses terminal-native background (transparent / no forced SGR), with panel-bg retained as opt-in
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: Issue 211; Issue 214; Issue 219; `spec/colors.yaml`; `spec/indicators.yaml`; `internal/usage/colorsspec.go`; `internal/usage/indicatorsspec.go`

---

## 1. Problem & Motivation

User request (verbatim): "make 'background' the default BG color for all charts."

Investigation of the current code confirms there is no "background" color today
— every chart surface (usage bars, sparklines, Braille load histories) resolves
its background to `panel-bg`, defined in `spec/colors.yaml` with a hardcoded
`sgr: "40"` (ANSI black background):

- `spec/colors.yaml`: `panel-bg` — "Shared black chart field selected by
  indicators.yaml", `sgr: "40"`.
- `spec/indicators.yaml:97-100`: "All chart surfaces resolve this one named
  background from spec/colors.yaml." → `chart-background: panel-bg`.
- `internal/usage/colorsspec.go` `init()`: wires `rograph.DefaultBackgroundANSI
  = colorSGR("panel-bg")` — i.e. every `rograph.RenderBar`/`RenderSparkline`
  call that leaves `BackgroundANSI` empty gets this forced-black default.

Issue 211 (In Review) already established the *mechanism* for a single,
spec-owned chart background token shared by every chart surface — that part
of the ask is done. What issue 211 did not address, and what this ticket is
about, is the *value* of that token: it is unconditionally black (`sgr 40`)
regardless of the user's terminal theme. On a light-background terminal this
paints a black rectangle behind every chart instead of blending with the
terminal's own background.

The literal ask — make "background" (i.e. the terminal's own/native
background, no explicit SGR background override) the default — reads as a
request to add a `background` option that means "don't force any background
color," and make chart surfaces default to it instead of `panel-bg`.

## 2. Scope

- Add a `background` color entry (or equivalent no-op/pass-through sentinel)
  to `spec/colors.yaml` and to whatever validation `internal/usage/colorsspec.go`
  applies to color entries, that resolves to *no* ANSI background SGR code
  being emitted (letting the terminal's own background show through), rather
  than to a specific SGR value. `time-gauge-bg` already has a documented
  precedent for a color entry with no `sgr` field
  (`internal/usage/colorsspec.go` explicitly exempts `panel-bg` and
  `time-gauge-bg` from the "missing sgr" validation error) — reuse or extend
  that pattern rather than inventing a second one.
- Change `spec/indicators.yaml`'s `chart-background` default from `panel-bg`
  to `background` (or update `internal/usage/colorsspec.go`'s
  `rograph.DefaultBackgroundANSI` wiring, whichever is the correct seam per
  issue 211's design) so every chart surface picks up the terminal-native
  background by default.
- Preserve `panel-bg` as a selectable, non-default value — do not delete it —
  since some existing/planned presentations (e.g. issue 211's btop-style heat
  mode, issue 214's 256-color heat palette) may still want an explicit black
  field for contrast; confirm with those tickets' authors/tests before
  assuming this change also applies to color-heat-mode charts.
- Out of scope: choosing new heat/foreground palettes (issues 211/214/219
  already own that), and anything about the live mic meter (tracked
  separately in issue 262).

## 3. Acceptance Criteria

- [ ] A `background` (terminal-native/no-op) color is defined in
      `spec/colors.yaml` and validated by `internal/usage/colorsspec.go`
      without requiring an `sgr` value.
- [ ] Default monochrome chart rendering (bars, sparklines, Braille load
      histories) no longer forces a black ANSI background — verified by a
      unit test asserting the rendered escape sequence contains no `40`
      background SGR code (or equivalent) for the default presentation.
- [ ] `panel-bg` remains available and unaffected for any presentation mode
      that explicitly opts into it.
- [ ] `spec/indicators.yaml`'s `chart-background` documentation comment is
      updated to describe the new default and how to opt back into
      `panel-bg`.
- [ ] Manual verification: `harnez usage --watch` visually confirmed on a
      light-theme terminal to no longer paint a black background behind
      charts by default.

## 4. Verification Guidance

- Unit tests around `internal/usage/colorsspec.go`'s `parseWatchColorsYAML`
  and `colorSGR`/`ansiOpen` for the new `background` entry (no `sgr`, no
  emitted background escape code).
- `go test ./internal/usage/...` for existing chart-rendering assertions that
  currently assume `panel-bg`/`sgr 40` — update any that hardcode this
  expectation.
- Live/manual check in an actual terminal (not just a test harness) with a
  light color scheme, since ANSI background behavior is exactly the kind of
  thing unit tests alone can silently get "passing" while still looking wrong
  to a real user.
