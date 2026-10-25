# 949 — 942 D: harnez-models update skill, cheap cited web refresh of public model facts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic), 941 (decisions 1, 3, 4 carry over), 945, 946, 948

---

/goal `docs/commands/harnez-models.md` (installed by `harnez apply`) runs `harnez-models update`: cheap advisor
searchers in parallel, one angle each (versions, costs subscription and API, capabilities, pitfalls, new
models), every finding with a source link; shows the diff and writes only on owner confirmation. In the harnez
repo it updates the shipped spec; elsewhere it writes confirmed public facts to the state dir, never into the
user's `models.yaml`. It then runs access discovery (946) and the change report (948). Or stop and report on a
design choice. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.
