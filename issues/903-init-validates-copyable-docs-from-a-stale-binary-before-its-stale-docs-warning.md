# 903 — init validates copyable docs from a stale binary before its stale-docs warning

**Status**: Backlog
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 891 and 790 (stale-docs warning), docs/CommandsPipeline.md (Stale-binary pitfall)

---

/goal In the harnez source repo, `harnez init -d .` with a stale installed binary stops with one clear
"installed binary is stale, run make install" error before any copyable-doc validation or copy, covered by
a test; other repos are unaffected. Stop and report if detecting the source repo needs an owner decision.
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Repro (2026-10-24)

Edited `docs/practices/AgenticLoop.md` to fix a line the copyable-doc destination check rejects, then ran
`harnez init -d .` without `make install`. Init kept failing with "assumes project destination
docs/studies/" for the already-fixed text (validation in `internal/claude/init.go` ~line 404 reads the
embedded source). The 891 stale-docs warning never appeared because validation errors first.

## Notes

- Order fix: run the stale check (embedded vs on-disk `config.yaml` and doc sources) before validation.
- Scope: only when the target dir is the harnez source repo (`go.mod` module `ubunatic.com/harnez`).
