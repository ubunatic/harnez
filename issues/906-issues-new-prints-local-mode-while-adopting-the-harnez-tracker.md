# 906 — issues new prints local mode while adopting the harnez tracker

**Status**: Backlog
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 901 (tracker adoption), 902 (found in the snapdex new-project run)

---

/goal The first `harnez issues new` in a fresh repo reports only that it adopted the harnez tracker; it no
longer prints "local mode" in the same run, covered by a test on the output. Stop and report if "local
mode" means something else that must stay. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## Repro (2026-10-24, snapdex)

```
issues new: local mode
issues new: adopted file-based tracker (tracker: harnez)
```

## Jules audit (2026-11-10, af9688e0)

Still valid. `cmd/harnez/issues.go:401` prints "issues new: local mode" unconditionally before the local
flow; the adoption line comes from `cmd/harnez/issues.go:783`. The test
`TestRunIssuesNewAdoptsOnlyNewIssuesDirectory` (`cmd/harnez/issues_test.go:744`) asserts that "local mode"
is printed in the fresh-repo case, so the fix must change that assertion. Report: `reports/jules/906.md`
(github#150).
