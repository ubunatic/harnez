# 948 — 942 F: model snapshots and change report

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic), 945, 946, 947, 949

---

Re-cut 2026-10-25 (epic 942): slice F1 first: on `harnez models discover`, save a snapshot of the effective
catalog and print a text report of models added/removed and access changes against the previous snapshot. The
JSON form and the other categories below follow as later milestones. 945's snapshot/report schemas are
provisional and may change.

Original /goal (full scope): Each model update saves an immutable snapshot of the effective catalog and prints a change report against
the previous one: models added or removed, version changes, cost up or down, capabilities gained, assumptions
proven or disproven, access changes, configuration errors; rows for humans and JSON
`{category, model, field, before, after, sources, evidence, severity}`, with snapshot IDs and discovery
completeness; proposed and applied changes kept apart. Tests cover every category, an interrupted update and
concurrent writers. Or stop and report on a format choice. Circuit breaker: on a loop or no progress, clear the
goal and write a failure report here.
