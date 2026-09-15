# 346 — `harnez status` false-positive "unindexed ticket" drift for titles with punctuation

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [036 — harnez status Issues Tracker Status Linter & Reconciliation](036-harnez-status-issues-tracker-linter.md), `harnez index` / `harnez index --check`

---

## 1. Problem & Motivation

In `~/projects/loom`, `harnez status` reports:

```
Issues:
  tracker:       issues/README.md [53 tickets: 52 ok, 1 drift]
    ! unindexed ticket file '040-treemap-theming-theme-1-current-vs-theme-2-quad-halfblock-sub-cell-rendering.md' not present in README table
```

This is a false positive. In the same repo, with no changes in between:

- `harnez index --check` reports both `issues/README.md` and `docs/README.md`
  up to date (no drift).
- `grep -n "040-treemap" issues/README.md` confirms the row is present,
  correctly linking the exact filename and rendering the exact title.

So the authoritative regenerator (`harnez index`, per ticket 148) and the
`harnez status` drift linter (per ticket 036) disagree about the same file,
and `harnez index --check` is right — the row genuinely exists.

The likely cause: ticket 040's title contains punctuation the two checkers
may tokenize/match differently —

```
Treemap theming: theme 1 (current) vs. theme 2 (quad/halfblock sub-cell rendering)
```

— specifically a colon, nested parentheses, and a `/`. `harnez status`'s
"is this ticket file's basename present in the README table" check appears
to break on one of these characters (or on the combination), while
`harnez index`'s regenerator handles the same title correctly when
(re)writing the table.

## 2. Proposed Solution

- Reproduce locally: create a scratch ticket whose title includes a colon,
  parens, and a slash (mirroring loom's 040), run `harnez index` to write
  a correct table row for it, then run `harnez status` and confirm the
  false positive reproduces.
- Align `harnez status`'s "ticket present in README table" match logic with
  whatever `harnez index --check` already uses to diff correctly — ideally
  by sharing the same parsing routine rather than maintaining two divergent
  implementations that can silently disagree (that disagreement is the core
  defect, independent of which one is "more correct" for punctuation).
- Add a regression fixture: a ticket title containing `:`, `()`, and `/` in
  the punctuation/drift test suite from ticket 036, so future divergence
  between `harnez index` and `harnez status` is caught by tests instead of
  by a user report.

## 3. Verification & Acceptance

- `harnez status` and `harnez index --check` agree (no drift reported by
  either) for loom's ticket 040 without modifying loom's `issues/README.md`.
- New unit/regression test covering a ticket title with colon/parens/slash
  passes.
- Existing `harnez status` and `harnez index` test suites continue to pass
  unchanged.
