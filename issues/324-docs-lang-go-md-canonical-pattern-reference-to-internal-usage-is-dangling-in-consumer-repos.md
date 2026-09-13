# 324 — docs/lang/Go.md canonical-pattern reference to internal/usage is dangling in consumer repos

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/lang/Go.md`, issue 166, issue 249

---

## Summary

Found while dogfooding `harnez init` in the `loom` consumer repo (2026-09-13).
`docs/lang/Go.md`'s new "Strings, Runes & Terminal Width" section (added per issue
166) ends with:

```markdown
- The existing `internal/usage` width assertions are the canonical pattern for TUI output.
```

`internal/usage` is a package that exists in the harnez repo itself
(`internal/usage/indicatorsspec.go`, `internal/usage/watch_test.go`, per issue 166)
but does not exist in consumer repos this doc is copied into — confirmed absent in
`loom` after this session's `harnez init` (`find . -type d -iname usage` → no
results). An agent reading the copied `docs/Go.md` in `loom` (or any other consumer
project) cannot follow this reference; it points at a directory that was never
copied and never will be, since it's harnez's own internal implementation, not a
bundled doc.

## Root cause

Same class of problem as issue 249 ("Copied Practice Docs Retain Dangling and
Inapplicable Dependencies") — a copyable/bundled doc's body references
harnez-repo-internal implementation details as if they were universally present in
every consuming project.

## Proposed fix

- Rephrase the bullet to describe the pattern itself (rune-based counting +
  `runewidth.StringWidth` + ANSI stripping before measuring) without pointing at a
  path that only resolves inside the harnez repo, e.g.: "Use rune-based counting and
  `runewidth.StringWidth` after stripping ANSI escapes; write width assertions
  against rendered output, not rune count." Optionally keep the `internal/usage`
  pointer but scope it explicitly as "(see harnez's own `internal/usage` package for
  a worked example, if replicating this pattern in a new project)" so it reads as an
  illustrative aside rather than a load-bearing reference.

## Verification

- [ ] `docs/lang/Go.md`'s Strings/Runes/Terminal-Width section no longer instructs
      readers to consult a path that doesn't exist outside the harnez repo.
- [ ] `harnez init` in a consumer repo (e.g. `loom`) propagates the corrected text,
      confirmed via `git diff docs/Go.md`.
