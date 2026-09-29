# 637 — diff and status report drift in bundled skill copies

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[631-external-skills-canary-then-user-registry-b-then-vendored-skills-a]]

---

/goal `harnez diff` and `harnez status` show when an installed bundled skill copy differs from
the embedded one, with a test; or stop and report when blocked on a user decision.

## Problem
Bundled skills (`config.yaml` `bundled_skills`, `internal/skillreg/bundled.go`) are installed by
`apply`, which rewrites changed copies. `diff`/`status` skip them, so a local edit is invisible
until the next `apply` overwrites it silently.

## Fix idea
Reuse `sameTree` from `SyncBundled` in the diff/status path and list differing copies.
