# 159 — Spec-Driven Enable/Disable of Bar/Graph `[]` Brackets

**Status**: Closed — resolved in aff9b98 (bar bracket wrapper only; see scope note below)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `internal/rograph/options.go` (`BarOptions.Left`/`Right`/`NoWrapper`, already code-level
  toggles), [[157-spec-driven-usage-watch-chart-glyphs]] (made chart glyphs spec-driven but did not
  extend to bracket wrapping), [[136-bar-ansi-background-bracket-leak-and-color-spec]],
  [[137-rograph-library-boundary-and-full-color-spec-audit]] (established the `spec/`
  resolve-in-`internal/usage`-then-pass-plain-values-into-`rograph` layering this ticket should
  reuse), `spec/indicators.yaml`

## Problem

`rograph.BarOptions` already supports omitting or customizing the wrapping brackets around a bar
(`Left`, `Right`, `NoWrapper`), but no caller ever sets these from a spec — every bar in
`internal/usage/watch.go` renders with the hardcoded default `[` / `]`. There is no static,
spec-file-level way to turn bracket wrapping on or off for the watch panel's bars/sparklines; it
would currently require a Go code change and rebuild.

This is explicitly about a **static** spec setting (read once at startup/spec-load time, like
`spec/indicators.yaml`'s glyph frames), not a runtime toggle or CLI flag — no dynamic
enable/disable behavior is in scope here.

## Desired Outcome

Add a spec-driven setting (new key in an existing `spec/*.yaml` file, or a new one if none fits)
that controls whether bar/sparkline graphs render their `[]` wrapper, following the established
`spec/` bootstrap pattern (embed/parse/validate, resolved in `internal/usage` and passed into
`rograph` as plain `BarOptions`/`SparklineOptions` values — `rograph` itself must stay
spec-agnostic per its package-boundary rule from issue 137).

## Acceptance Criteria

- [x] A spec file exposes a bracket on/off (and optionally custom left/right glyph) setting.
- [x] `internal/usage`'s spec loader resolves this into `rograph.BarOptions` (`Left`/`Right`/
      `NoWrapper`) for the watch panel's bar call sites. (Sparkline call sites out of scope —
      see note below.)
- [x] `rograph` package itself is not touched beyond what's already exposed — no new
      spec-awareness added to `internal/rograph`.
- [x] Default behavior (brackets on, `[`/`]`) is unchanged unless the spec is edited.
- [x] Tests cover both the bracketed default and a brackets-disabled spec value.

## Scope Note (added at close)

Sparklines were dropped from scope after implementation research. `rograph.SparklineOptions`
has no `Left`/`Right`/`NoWrapper` fields — only `BarOptions` exposes bracket-wrapper toggles.
Adding matching fields to `SparklineOptions` would satisfy the letter of AC2's "and
`SparklineOptions`" language but directly conflicts with AC3 ("`rograph` package itself is not
touched beyond what's already exposed"), since that's new API surface, not existing surface
wired up.

Separately, the `[`/`]` brackets drawn around watch-panel sparklines (`formatCPULine`,
`formatGPULine` in `internal/usage/watch.go`, plus the token-velocity spark line and
`internal/usage/history.go`'s per-model spark line) are hardcoded literal characters inside each
call site's own `fmt.Sprintf` format string, not routed through any single shared helper the way
every bar call site already routes through `watchBarOptions()`. Making those spec-driven too
would mean editing several independent format strings across two files rather than one shared
resolver — a reasonable follow-up, but disproportionate scope for this P3 ticket and better
tracked as its own ticket if wanted.

Bars were fully in scope and are now spec-driven: every `rograph.RenderBar` call site in
`internal/usage/watch.go` already goes through the shared `watchBarOptions()` helper, so wiring
the spec there covers all of them in one place.
