# 162 — `internal/issues.ParseBody`: Three-Tier Fallback for Body Extraction

**Status**: Closed — resolved in c11465f
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[158-find-entity-query-command]] (introduced `ParseBody`; its Scope Note flagged this
  gap at close), `internal/issues/issues.go`

## Problem

`ParseBody` currently extracts the searchable body as "everything after the first metadata-closing
thematic break (`---`/`***`/`___`)". If no such break exists, `Body` is `""` — the ticket becomes
title-only searchable via `harnez find`. This was accepted as a known limitation when 158 closed.

A repo-wide check across all `*/issues` trees (705 tickets, 25 projects) confirmed:

- 371 tickets lack the `---` rule.
- Of those, 0 have `###` as their first heading (no heading-level-skip risk).
- 6 have **no** `## ` heading at all (free-text/init-style tickets, e.g. `proctop/issues/000-init.md`).
- Within harnez's own tracker specifically (154 tickets, 88 without `---`), all 88 have a `## `
  heading — zero edge cases today.

So the two-tier fallback (`---` → first `## `) is safe for every ticket `find` ships against by
default, but `find -d <other-repo>` can hit one of the 6 no-heading files and silently get an empty
body under a naive "must find `## `" rule.

## Desired Outcome

Extend `ParseBody`'s fallback chain to three tiers, in order:

1. Everything after the first `---`/`***`/`___` thematic break (current behavior, unchanged).
2. Else, everything after the first `## ` heading.
3. Else, everything after the H1 title (i.e. the whole remaining document is the body).

This removes the empty-body edge case entirely rather than relying on it not occurring in practice.

## Acceptance Criteria

- [x] `ParseBody` implements the three-tier fallback above.
- [x] Tests cover all three tiers, including a fixture with no `---` and no `## ` (mirroring the
      no-heading files found in the repo-wide check).
- [x] Existing 158 test coverage (body search, title-only degradation) still passes.
- [x] `go test ./...`, `make install`, `harnez status` clean.
- [x] Update 158's Scope Note or this ticket to reflect the fallback is now three-tier, not two.

## Out of Scope

- Any change to the `---`-first-tier semantics.
- Migrating any ticket file to add a `---` or `## ` heading — this is a code-side fallback only.

## Scope Note (added at close)

- **Tier selection is "first match wins" per document, not per line-position tie-break.** A single
  forward scan (starting after the H1 title) tracks the first thematic break index and the first
  `## ` heading index independently; if a thematic break is found the scan returns immediately
  (tier 1 short-circuits before a later `## ` could ever matter). If no thematic break exists at
  all, the earlier-recorded `## ` index (if any) wins for tier 2. There's no scenario where a `##
  ` heading appears "on the very last line with nothing after it" needs special handling beyond
  what the code already does naturally: `lines[headingIdx+1:]` on a heading that's the last line
  is a zero-length slice, joining to `""` — an empty but well-defined body, not an error.
- **Tier 3 empty-title-only case.** `ParseBody` still returns `""` when no H1 title is found at
  all (unchanged from before this ticket) — this wasn't in scope to fix, since 162's fallback
  chain is explicitly anchored on "the H1 title" as the tier-3 base case; a title-less document
  has no tier to fall back to.
- **158's title-only degradation is now resolved.** With this fallback, the only remaining
  title-only-search case is a document with no H1 at all, which is a distinct, unaddressed
  malformed-ticket scenario, not the `---`-less case 158 originally flagged.
