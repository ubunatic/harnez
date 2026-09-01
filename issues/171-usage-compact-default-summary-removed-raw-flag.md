# 171 — `harnez usage`: Compact Dashboard Is Now the Default, `--summary` Removed, `--raw` Added

**Status**: Closed — resolved in 5c8b5f6 (see commit for exact hash after this file is committed)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[102-usage-summary-compact-should-be-allowed]] (closed — its `--summary`/`--compact`
combination no longer exists as such, since `--summary` is gone and `--compact` now applies to the
default view directly), `cmd/harnez/main.go` (`validateUsageFlags`, `usageCmd` RunE and flags)

---

## 1. Problem & Motivation

Two pieces of user feedback in one session:

1. `harnez usage --compact` errored (`--compact requires --watch or --summary`) even though
   `--compact`'s own help text already read "start `--watch` with only all-usage and load panels
   visible" — the user expected it to just work.
2. Follow-up direction, once that was fixed: rather than have `--compact` imply `--watch` (a
   live-refreshing loop), the user explicitly said **not** to imply `--watch` — `--compact` should
   just show the compact output once. Taking that further, the user then asked to make the compact
   one-shot dashboard (previously reached via `--summary`) the **default** output of a bare
   `harnez usage`, **remove the `--summary` flag entirely**, and add a new **opt-in `--raw`/`-r`
   flag** for what used to be the unconditional flat/detailed per-field text report.

## 2. What Changed

`cmd/harnez/main.go`:

- **`--summary`/`-s` flag removed.** A bare `harnez usage` (no flags) now prints the compact
  btop-style dashboard once and exits — exactly what `--summary` used to do.
- **`--raw`/`-r` flag added**, opt-in: prints the detailed per-field text report (`RenderText`) that
  used to be the unconditional default before this change.
- **`--compact` no longer requires or implies anything.** It's a pure panel-selection toggle
  (`compactWatchSections()` vs. the default panel set) that now applies to both the default
  one-shot view and `--watch`. `harnez usage --compact` alone works (fixes point 1 above) without
  starting a live-refresh loop (per point 2).
- **`--json`** is unaffected as an orthogonal output-format flag; it still fully bypasses both the
  compact and raw renderers, unchanged from before.
- **New conflicts**: `--watch`+`--raw`, `--raw`+`--json`, and `--compact`+`--raw` (the flat report
  has no panel concept to toggle) are all rejected by `validateUsageFlags`. `--watch`+`--json` was
  already rejected before this change and still is.

`validateUsageFlags`'s signature changed from `(usageWatch, usageSummary, usageCompact)` to
`(usageWatch, usageRaw, usageJSON, usageCompact)`.

## 3. Compatibility Note

This is a **breaking CLI change**: any script or muscle-memory habit invoking `harnez usage
--summary` now gets `Error: unknown flag: --summary` and must drop the flag (the same output is now
the default) or migrate scripts that wanted the old flat text report to `harnez usage --raw`.
Solo/hobby repo, no external users to coordinate with (per this project's Repo Setup convention),
so no deprecation shim was added — the flag was removed outright rather than kept as a silent alias.

Historical docs/issues (e.g. issue 102, `docs/studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md`,
and others found via `grep -rn -- '--summary' issues/ docs/`) still reference `--summary` by name —
those are accurate historical records of what was true when written and were deliberately left
unedited; they are not live documentation of current flag behavior.

## 4. Verification

- `go build ./...`, `go vet ./...` clean.
- `go test ./...` (full repo) passes; `cmd/harnez/usage_test.go`'s `validateUsageFlags` tests rewritten
  for the new signature and semantics (no-flags OK, `--compact` alone OK, `--compact`+`--watch` OK,
  `--watch`+`--json`/`--watch`+`--raw`/`--raw`+`--json`/`--compact`+`--raw` all rejected, `--raw`
  alone OK).
- Live-verified all four cases: bare `harnez usage` shows the compact dashboard; `--compact` shows
  the reduced-panel one-shot view; `--raw` shows the old flat report; `--compact --raw` and the now-
  removed `--summary` both correctly error.
- `make install` run.
