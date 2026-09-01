# 168 — Splash: Determinate Left-to-Right Progress Bar Driven by a Fetch-Duration Estimate

**Status**: Closed — resolved in b67a7ee
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[164-fast-startup-usage-watch-splash-or-stale-data]] (resolved in `a1178d4` — this
ticket replaces that implementation's indeterminate sweep bar with a determinate one), `spec/colors.yaml`,
`spec/indicators.yaml`, `internal/usage/watch.go` (`buildSplashFrame`, `splashBarPercent`,
`splashSpinnerGlyph`), `internal/usage/statecache.go`, `internal/usage/livefetchcache.go`

---

## 1. Problem & Motivation

Issue 164 shipped a splash screen for `harnez usage --watch` startup, but its progress bar
(`splashBarPercent`, `internal/usage/watch.go:1742-1753`) sweeps 0→100→0 on a fixed
`splashBarSweepPeriod` (1200ms) — a pulsing/indeterminate bar, not a real progress indicator. The
user wants a genuine determinate bar: starts empty at the left, fills monotonically toward the
right, and reaches ~100% around when the first live fetch (`CollectAll`/`CollectRemote`) actually
completes.

This requires an estimate of how long the pending fetch will take. The user's framing: "we should
know that from our experience" — i.e. derive the estimate from this project's own observed fetch
durations rather than a fixed guess, since different fetch paths (local `CollectAll` across
multiple quota sources vs. remote `CollectRemote` over SSH) have genuinely different typical
durations.

## 2. Technical Specification / Findings

- **No existing timing data**: neither `internal/usage/statecache.go` nor
  `internal/usage/livefetchcache.go` currently records how long a fetch actually took — only
  `FetchedAt` (when it completed) is tracked (`statecache.go:61 IsFresh`). A duration estimate
  needs a new persisted signal: wall-clock elapsed time of each `CollectAll`/`CollectRemote` call
  (or ideally, of each per-agent/per-source sub-fetch inside `CollectAll`, since a single slow quota
  source shouldn't be masked by averaging against fast ones).
- **Estimate source options** (pick one, don't over-build):
  1. A simple rolling estimate (e.g. EWMA or "last N durations, averaged") persisted alongside the
     existing on-disk quota cache (`statecache.go`/`livefetchcache.go` machinery), keyed by fetch
     kind (local vs. `CollectRemote` per host) so estimates don't get cross-contaminated between a
     fast local fetch and a slower SSH-tunneled remote one.
  2. A fixed, spec-driven default duration (see styling/spec note below) used only until enough
     real samples exist, then blended with or replaced by the rolling estimate.
- **Bar behavior once an estimate exists**: `pct = min(100, 100 * elapsed / estimatedDuration)`,
  capped below 100% (e.g. 95%) until the fetch actually completes, so the bar never visually
  "finishes" before the real data is ready — avoids a confusing stall at 100% while still waiting.
  If no estimate is available yet (true cold start, first-ever run), fall back to the existing
  indeterminate sweep behavior from issue 164 rather than showing a fabricated determinate bar.
- **Styling stays spec-driven** (per project convention — see issue 164's implementation, which
  reused `spec/indicators.yaml`'s `usage-bar` glyph set and `spec/colors.yaml`'s `bold`/`dim-grey`
  via `watchBarOptions()`/`rograph.RenderBar`/`ansiWrap` rather than hardcoding ANSI/glyphs). This
  ticket must follow the same discipline:
  - Reuse `watchBarOptions()`/`rograph.RenderBar` for rendering the bar itself — only the `pct`
    calculation changes, not the rendering path.
  - Any fixed default/fallback duration constant (see estimate option 2 above) belongs in a spec
    file (most likely a small addition to an existing spec, e.g. `spec/indicators.yaml` or a new
    lightweight spec, not a Go source literal) if it needs to be a tunable, named value — check
    with the spec/colors.yaml and spec/indicators.yaml loader patterns
    (`internal/usage/colorsspec.go`, `internal/usage/indicatorsspec.go`) before adding a new spec
    file; reuse the existing loader machinery rather than inventing a new one if the shape fits.

## 3. Implementation & Verification Plan

- Add duration recording to the fetch path(s) (`CollectAll`/`CollectRemote` call sites in
  `RunWatchWithOptions`, `internal/usage/watch.go`) and persist a rolling estimate keyed by fetch
  kind, using the existing on-disk cache directory/locking machinery
  (`internal/usage/statecache.go`, `internal/usage/livefetchcache.go`) rather than a new storage
  mechanism.
- Replace `splashBarPercent`'s sweep math with a determinate calculation driven by the persisted
  estimate when available, falling back to the existing sweep when no estimate exists yet (first
  run on a given host/fetch-kind).
- Keep the spinner (`splashSpinnerGlyph`) as-is — the user's feedback is specifically about the bar
  being pulsing rather than determinate, not about the spinner.
- Verify with a pty-based repro (per this project's TUI-bug pattern — reasoning-only fixes for
  ANSI/terminal bugs have failed before): confirm the bar fills left-to-right and reaches ~95% near
  actual completion on a warm estimate, and confirm the fallback sweep still works on a genuinely
  cold cache with no estimate yet.
- Unit-test the percent calculation (`elapsed/estimate` math, capping behavior, fallback-to-sweep
  branch) as a pure function, following the existing `dispatchSplashKey`/`buildSplashFrame` test
  pattern from issue 164's `watch_test.go`.
