# 877 — practice: new features go into sub-packages, not the main command or root package

**Status**: Closed — Bundled Go guidance directs substantive features and tests into internal sub-packages and keeps cmd thin; full suite passes.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Practice
**Related**: loom 377, loom 378, docs/Go.md

---

/goal The bundled Go practice (docs/Go.md or a new section) tells agents to put a new feature in its
own package (`internal/<feature>` by default) with its tests, leaving only a thin command in `cmd/<app>`,
and explains the test-cache benefit; or stop and report when blocked on a user decision.

## Finding (loom, 2026-10-23)

Agents kept adding features as files to `cmd/loom` (session 1,266 lines, editor 998) and the root
package (118 files, 29k lines). Go caches test results per package, so any change re-ran most of the
suite (28 of 39 packages). Moving session and editor to `internal/` (loom 378) was a quick pure move.

## Proposal

- New feature → own package; `cmd/<app>` keeps flags, help, exit codes.
- `internal/` unless a public API is intended (then API stability rules apply).
- Signal for a split: a file over ~800 lines, or a package whose change re-runs most of the suite.
