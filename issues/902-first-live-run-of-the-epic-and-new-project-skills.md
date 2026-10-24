# 902 — first live run of the epic and new-project skills

**Status**: Backlog
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 899 (new-project skill), 900 (epic skill), 901 (tracker detection bug found in the dry run)

---

/goal Use `/new-project` and `/epic` once each on real work and fold the friction into
docs/commands/new-project.md and docs/commands/epic.md; or stop and report when no real project or epic is
due. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Scope

899 was verified by a scratch dry run of the local steps (git init, harnez init per stack, Makefile no-ops,
first commit, first ticket); the forge steps (`fj`/`gh repo create`) need owner confirmation and were not
run. 900 is built from the 894 run; no new epic was run with it yet. Record what each live run changed.

## Run 1: /new-project snapdex (2026-10-24)

Local steps ran end to end (snapdex ff24880, c2a705b); the 901 fix held (`tracker: harnez` after the first
ticket, merge driver on re-init). Folded into new-project.md: REUSE licensing (`LICENSES/` + `REUSE.toml`)
instead of a bare LICENSE so `make reuse` passes; set Status/Category on the Draft/Bug template and run
`harnez index` before committing. Open, not fixed: `issues new` prints "local mode" in the same run that
adopts the harnez tracker (misleading). Forge steps and `/epic 1` refinement are still pending.
