# 218 — Fix heat-mode usage alignment and remote Braille chart widths

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: Issue 211; Issue 213; commits `878f83d`, `49b2311`, `77fc20b`, `5b924e4`

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
