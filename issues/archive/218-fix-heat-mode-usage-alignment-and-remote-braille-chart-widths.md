# 218 — Fix heat-mode usage alignment and remote Braille chart widths

**Status**: Closed — resolved in `bf51b25` and `f0f5cef`
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: Issue 211; Issue 213; commits `878f83d`, `49b2311`, `77fc20b`, `5b924e4`, `e232aee`, `bf51b25`, `f0f5cef`

---

## 1. Problem & Motivation

Recent heat-presentation and btop-default changes regress live `usage` layout:

- In the All Usage section, a quota percentage reaching `100%` shifts the
  area to the right by one terminal cell instead of retaining its fixed column
  alignment.
- Remote CPU and remote GPU/Phoenix history charts render shorter than their
  local counterparts and no longer align with the other CPU/GPU charts.

These are visible terminal-layout regressions. ANSI styling and doubled
Braille sample resolution must never change the visual width budget or column
alignment of the dashboard.

## 2. Required Investigation

- Audit the recent heat-mode usage-bar formatting changes, including all
  percentage-label padding and ANSI-aware width calculations. Establish why a
  three-character `100%` value affects a fixed layout.
- Audit remote load-history collection/padding and its rendering width after
  the two-samples-per-Braille-cell change. Establish why remote CPU/GPU charts
  receive fewer display cells than their local equivalents.
- Identify the exact introducing change(s); do not mask either symptom with
  hardcoded spacing.

## 3. Acceptance Criteria

- All Usage quota rows retain identical visible column positions for `0%`,
  `99%`, and `100%`, in monochrome and heat modes, at normal and compact
  widths.
- Remote CPU and GPU/Phoenix charts have the same declared visible width as
  equivalent local charts in the same layout mode, including with short and
  fully populated histories.
- Tests measure ANSI-stripped terminal width and alignment rather than raw
  string byte length, covering heat foreground/background sequences.
- Existing local chart widths, btop Braille geometry, and label content remain
  intact.
- Run targeted tests, `go test ./...`, `make check`, `make install`, and a
  visual `harnez usage --watch` check before closure.

## 4. Implementation Checkpoint

- Quota-layout scope implemented in `e232aee`, ready for review.
- Root cause: the narrowest All Usage fallback emitted the first styled
  percentage without its former fixed-width field. `100%` therefore occupied
  one more terminal cell than `99%` and shifted the second quota bar.
- The fallback now pads by ANSI-stripped display width, with invariant tests
  for `0%`, `99%`, `100%`, missing durations, and single-window data at compact
  and full widths in monochrome and heat modes.
- Verified with targeted usage tests, `make check`, and `make install`.
- Remote CPU/GPU Braille-width investigation remains open and was explicitly
  excluded from this implementation scope.

## 5. Quota Alignment Follow-up

- Commit `bf51b25` fixes the remaining wide-duration path exposed by the live
  `Claude/GPT` row (`100% 22h30m`). The prior 10-cell middle field covered
  `99% 22h30m` but overflowed by one display cell at `100%`.
- All Usage now derives one shared middle-column width from the longest first
  quota duration in the table, while reserving four display cells for every
  percentage through `100%`. This keeps every second quota bar on the same
  terminal column without special-casing a duration or percentage.
- ANSI-stripped display-column tests cover `0%`, `99%`, and `100%` with real
  durations (`8h51m`, `6d2h`, and `22h30m`) in monochrome and heat modes at
  All Usage box widths 51, 55, 60, 80, and 100. The same matrix covers the
  single-window placeholder path.
- Verified with targeted regression tests, `go test ./...`, `make check`, and
  `make install`.

## 6. Remote Braille Width Resolution

- Commit `f0f5cef` fixes the remaining remote CPU and GPU/Phoenix chart
  width regression. Remote snapshots can contain short histories while a
  stream warms up or when batch collection restarts the remote process.
- The doubled Braille resolution consumes two samples per display cell, but
  CPU/GPU formatters derived their requested chart width directly from the
  retained sample count. A 10-sample remote history therefore rendered only
  five cells while a fully seeded 20-sample local history rendered ten.
- CPU and GPU histories now use the existing history-padding policy before
  requesting the shared ten-cell chart width. Padding repeats the oldest
  available observation and retains all real samples, matching the existing
  RAM and VRAM/GTT behavior without hardcoded visual spaces.
- ANSI-stripped tests compare remote short/full histories with fully seeded
  local counterparts for CPU and GPU/Phoenix rows at compact width 34 and
  normal width 48. They assert intrinsic chart width and column bounds plus
  rendered panel width/alignment.
- Verified with targeted regression tests, `go test ./...`, `make check`, and
  `make install`; heat presentation and btop defaults remain unchanged.
