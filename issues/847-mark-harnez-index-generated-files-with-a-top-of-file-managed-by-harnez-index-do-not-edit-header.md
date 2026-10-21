# 847 — Mark harnez-index generated files with a top-of-file "managed by harnez index, do not edit" header

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 018 (bundled docs self-identify in front matter), 148 (studies table), 237, 846 (Roadmap labels)

---

/goal Every file (or section) that `harnez index` regenerates carries a marker at the top that
agents see before editing, verified on this repo after `harnez index`, or stop and report when
blocked on a user decision.

## Problem

On 2026-10-21 the host hand-edited the studies table in `docs/README.md` twice; the next
`harnez index` overwrote both edits. A notice exists, but only at line ~97 above the studies
table, so an agent that reads the top or greps for a row never sees it. `issues/README.md` has
no such notice at its top either.

## Wanted

- Front matter or a first-line comment in `docs/README.md`, `issues/README.md` and any other
  file `harnez index` writes, e.g. `<!-- managed by harnez index: generated sections are
  overwritten; edit sources (study files, ticket headers) instead -->`, listing which sections
  are generated when the file is only partly generated (docs/README.md mixes hand-written
  evergreen rows with the generated studies/feedback tables).
- `harnez index` writes and keeps the marker; lint warns if it is missing.
- Consistent with the 018 front-matter convention and the roadmap block from 846.

## Open questions

- Which parts of `docs/README.md` are hand-written vs. generated? The marker must not
  discourage editing the hand-written evergreen table.
- Use `harnez:begin/end`-style block markers around generated sections, in addition to the
  top marker?
