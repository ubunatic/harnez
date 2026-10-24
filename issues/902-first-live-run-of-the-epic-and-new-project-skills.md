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
