# 847 — Mark harnez-index generated files with a top-of-file "managed by harnez index, do not edit" header

**Status**: Open — reopened: merge commit drops incoming rows
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

## Merge conflicts in generated files (user decision 2026-10-22)

843 M3 adds `issues/BACKLOG.md` and `issues/CLOSED.md`, so the set of generated files keeps growing.
Generated files must not cause manual merge conflicts:

- **Merge driver:** `.gitattributes` maps each generated file to `merge=harnez-index`; the driver
  regenerates the file from the sources (tickets, studies) and accepts the result. `harnez init`
  installs the driver in `.git/config`, since git only reads drivers from there.
- **Safety net:** regenerate after merge/rebase (hook or `harnez issues rebase`); lint fails when a
  committed generated file differs from a fresh `harnez index`.
- **Low churn:** keep one line per ticket in a stable sort order, with no volatile counts or long text in
  rows, so any conflicts that remain are rare and trivial.

## Open questions

- Which parts of `docs/README.md` are hand-written vs. generated? The marker must not
  discourage editing the hand-written evergreen table.
- Use `harnez:begin/end`-style block markers around generated sections, in addition to the
  top marker?

## Searcher findings (2026-10-22, s847 luna:low)

- Generated: `issues/README.md` (table only), `issues/BACKLOG.md`, `issues/CLOSED.md` (full),
  `docs/README.md` (studies table only, found via `**\`docs/studies/\`**` + `| File | Topic |`).
  Writers `internal/index/index.go:68,202,358-383`, caller `cmd/harnez/index.go:125`. Roadmap not generated.
- Block-marker helpers exist: `internal/markdown/markdown.go:12`, `sections.go:14`. 018 uses
  `managed-by: harnez` front matter.
- `harnez init` already writes `.gitattributes`, a local merge driver and a pre-commit hook
  (`internal/claude/init.go:1180`), but attributes cover only `issues/README.md`.
- Freshness: `issues lint` checks all three issue indexes (`cmd/harnez/issues_rebase.go:525`),
  `harnez index --check` exists; nothing checks the docs/README studies table.
- Output is deterministic (sorted, no timestamps/counts).

## Milestones

- **M1 — markers:** top-of-file notice in every generated file (one line; for partly generated files it
  names the generated sections); wrap generated sections in `harnez:begin/end`-style block markers via
  the existing helpers, with a migration that finds today's heading/table anchors once. Hand-written
  parts of `docs/README.md` stay editable and are named as such.
- **M2 — merge driver + freshness:** extend the init-installed `.gitattributes`/driver to BACKLOG.md,
  CLOSED.md and docs/README.md (driver regenerates, works for partly generated files); `index --check`
  and lint cover the docs/README studies table; regenerate this repo and verify a forced conflict
  resolves via the driver.

## M1/M2 delivered (66b8387b, 27503f72), host review FAILED (2026-10-22)

Notices and `.gitattributes` lines are correct in this repo. But the live check fails:
- Scratch repo (`git init`, one ticket, docs/README with a studies anchor) → `harnez init -d .` exit 0, then
  `harnez index`: no `merge.*` git config, no `merge=` lines in `.gitattributes`. A forced conflict on
  new tickets/studies on two branches leaves `<<<<<<<` in `docs/README.md` and `issues/README.md`.
- This clone (`/home/uwe/projects/harnez`) has no `merge.harnez-issues-index.*` config either, so the
  committed `.gitattributes` lines are inert here.

## M3 Pre-Work / Required Refinements

- Find out why `init` skips the driver (gating?) and make a plain `harnez init` on a repo with `issues/`
  install it; if it is opt-in by design, report the flag and make the opt-in path work end to end.
- Install it in this clone via the real CLI path, not by hand.
- Add an end-to-end test that runs the real init command (not the internal function), then a two-branch
  merge with new tickets and new studies, asserting no conflict markers in all four generated files.

## M3 delivered (fa6a7493), host review FAILED again (2026-10-22)

Plain `init` now installs the driver (verified in a scratch repo and in this clone). But a two-branch merge
(feat adds `002` + `studies/b.md`, main adds `003` + `studies/c.md`) commits generated files **without
the incoming branch's rows** (002, b.md): the driver regenerates from the working tree, where git has not
yet checked out the other side's new files. The post-merge hook then rewrites both READMEs, leaving the
tree dirty, and `issues lint --cached` in pre-commit blocks every later commit until someone regenerates.
`issues/README.md` also lacks a trailing newline after the end marker in the merged result.

## M4 Pre-Work / Required Refinements

- Driver must produce the final merged content itself: build rows from the merge result's sources
  (e.g. the union of ticket/study paths in `%A` + `%B` trees via `git ls-tree`/`MERGE_HEAD`, or union of
  the generated rows from `%A` and `%B` re-sorted canonically), so the merge commit already equals a
  fresh `harnez index` and post-merge leaves a clean tree.
- E2E test must assert on the **merge commit content** (`git show HEAD:issues/README.md`) and a clean
  `git status` after merge, not on the working tree after hooks ran.
- Trailing newline after the end marker.
- Hand-written conflict case: re-test from a clean state (my run was blocked by the bug above).
