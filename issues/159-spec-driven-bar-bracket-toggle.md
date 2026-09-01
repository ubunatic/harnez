# 159 — Spec-Driven Enable/Disable of Bar/Graph `[]` Brackets

**Status**: Open
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

- [ ] A spec file exposes a bracket on/off (and optionally custom left/right glyph) setting.
- [ ] `internal/usage`'s spec loader resolves this into `rograph.BarOptions`/`SparklineOptions`
      (`Left`/`Right`/`NoWrapper`) for the watch panel's bar and sparkline call sites.
- [ ] `rograph` package itself is not touched beyond what's already exposed — no new
      spec-awareness added to `internal/rograph`.
- [ ] Default behavior (brackets on, `[`/`]`) is unchanged unless the spec is edited.
- [ ] Tests cover both the bracketed default and a brackets-disabled spec value.
