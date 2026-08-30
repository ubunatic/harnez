# 102 — `harnez usage --summary --compact` should be allowed

**Status**: Closed — resolved in 1d98fb6
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `cmd/harnez/main.go` (flag validation), `internal/usage/watch.go` (`RenderSummary`, `buildWatchFrame`, `compactWatchSections`)

## Problem

`harnez usage --summary --compact` is currently rejected:

```
$ harnez usage --summary --compact
Error: --compact requires --watch
```

The rejection comes from an explicit guard in `cmd/harnez/main.go`:

```go
if usageWatch && usageSummary {
    return fmt.Errorf("--watch and --summary cannot be combined")
}
if usageCompact && !usageWatch {
    return fmt.Errorf("--compact requires --watch")
}
```

The second check only permits `--compact` when `--watch` is set, with no carve-out for
`--summary`.

## Why this looks unintentional

`--summary` isn't a distinct rendering mode — per its own flag help text
(`"print the compact --watch-style dashboard once and exit"`) and the `RenderSummary` doc
comment in `internal/usage/watch.go` ("prints one static frame of the same compact,
btop-style grid... It exists for `harnez usage --summary`: same at-a-glance ... keyboard
handling"), `--summary` renders exactly one static frame of the same grid that `--watch`
redraws repeatedly. Both code paths ultimately call `buildWatchFrame`, and
`compactWatchSections()` (the panel-selection function `--compact` is meant to toggle) is
generic over both call sites.

So the validation logic appears to have been written with only the `--watch` path in mind
(likely because `--compact` was introduced/wired at the same time as `--watch`), and never
extended to acknowledge that `--summary` shares the same renderer. As written, a user who
wants the reduced-panel layout for a single one-shot snapshot (rather than a live-refreshing
dashboard) has no way to request it — they must either accept the full multi-panel `--summary`
layout or switch to `--watch` (which keeps redrawing) just to get `--compact`'s reduced panel
set.

## Suggested direction

- Loosen the guard at `cmd/harnez/main.go:52-54` to accept `--compact` when either `--watch`
  or `--summary` is set (i.e. `if usageCompact && !usageWatch && !usageSummary`).
- Thread `usageCompact` into the `RenderSummary` / `RenderSummaryRemote` call sites (currently
  only `usageProcesses` is passed at `main.go:72,74`), analogous to how `WatchOptions.Compact`
  is passed into `RunWatchWithOptions` at `main.go:60-64`. This likely means adding a
  `Compact bool` parameter (or reusing `WatchOptions`) to `RenderSummary`/`RenderSummaryRemote`
  in `internal/usage/watch.go` so they can select `compactWatchSections()` instead of the
  default section set.
- Add/update a flag-combination test alongside the existing ones in `cmd/harnez` (or
  `internal/usage`) covering `--summary --compact`.
