# 323 — docs/lang/Bash.md hard-references docs/practices/AgenticLoop.md instead of @docs/AgenticLoop.md alias

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/lang/Bash.md`, `docs/lang/Make.md`, issue 299, issue 319

---

## Summary

Found while dogfooding `harnez init` in the `loom` consumer repo (2026-09-13).
`docs/lang/Bash.md` §7 ("Commands & Traps") contains a hardcoded source-repo-relative
path:

```markdown
This is about individual command invocations; for waiting on a long-running
background job instead, see `docs/practices/AgenticLoop.md`'s "Blocking sleep
Waits" and "Buffered Long-Running Output" anti-patterns.
```

In every consumer project, `harnez init` installs `AgenticLoop.md` directly at
`./docs/AgenticLoop.md`, not under a `docs/practices/` subdirectory — so this
reference is dangling as soon as the doc is copied out of the harnez repo. Confirmed
in `loom` after this session's `harnez init`: `docs/practices/` does not exist there.

## Root cause

This is the exact same bug class as issue 299 ("Issue B": `docs/lang/Make.md` had
the identical hardcoded `docs/practices/AgenticLoop.md` path, fixed there to use
`@docs/AgenticLoop.md`). The fix in 299 was scoped to Make.md only and did not add a
lint/grep check to catch the same pattern elsewhere. Issue 319 subsequently added new
prose to Bash.md that reintroduced the same broken-path pattern, since nothing
flagged it at review time.

## Proposed fix

- Change the reference in `docs/lang/Bash.md` from `docs/practices/AgenticLoop.md`
  to `@docs/AgenticLoop.md` (the alias form, consistent with how `docs/lang/Make.md`
  now does it post-299).
- Consider a `harnez lint-docs`/CI grep for the literal pattern
  `docs/practices/\|docs/other/\|docs/lang/` inside copyable doc bodies, so a
  source-repo-relative path reintroduced in any future doc edit is caught before merge
  rather than rediscovered per-consumer-repo.

## Verification

- [ ] `docs/lang/Bash.md` references `@docs/AgenticLoop.md` instead of the raw
      source-repo path.
- [ ] `harnez init` in a consumer repo (e.g. `loom`) propagates the corrected
      reference, confirmed via `git diff docs/Bash.md`.
