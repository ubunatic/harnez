# 882 — Per-repo active ticket cap in .harnez/repo.yaml (harnez: 30)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 843 (cap 20 introduced), 854 (`.harnez/repo.yaml`)

---

/goal Each repo can set its own active-ticket cap in `.harnez/repo.yaml` (`issues.active_cap`), falling back to
`issue_index.active_cap` in `spec/issues.yaml` (20); harnez itself uses 30; verified by a test for the override,
the fallback and an invalid value (0, negative), and the warning on this repo disappearing at 24 active; or stop
and report when blocked on a user decision.

## Problem (owner, 2026-10-23)

"Increase open count to 30 (can we do this per repo)". The cap is global today: `spec/issues.yaml:63`
(`active_cap: 20`), read in `internal/issues/status_spec.go:135-154`, warning in
`cmd/harnez/issues_rebase.go:588`; tests in `cmd/harnez/issues_rebase_test.go:200-207` expect "cap 20".

## Plan

- `.harnez/repo.yaml` schema and loader: optional `issues.active_cap` (integer >= 1).
- Cap lookup reads the repo value first, then the spec default; the warning names the source.
- Set `issues.active_cap: 30` in this repo's `.harnez/repo.yaml`; check `harnez init -d .` keeps it.
- Tests: override, fallback, invalid value rejected with a clear error.
