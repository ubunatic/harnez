# 1135 — Jules backlog batch: model discovery and access, 942 children (merge/close assessment)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 485, 946, 947, 948, 949, 954, 955, 1122

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 485 Autodetect the default agent model from 5h and weekly usage limits
- 946 942 C: model access discovery on explicit update (agy list, claude /model, bare probe calls)
- 947 942 E: capability evidence tied to model version from attributable tests
- 948 942 F: model snapshots and change report
- 949 942 D: harnez-models update skill, cheap cited web refresh of public model facts
- 954 942 C2: harnez models discover lists advertised models and shows an access column
- 955 942 C3: bounded batch bare probes record accessible/denied per model

Note: open ticket 1122 (quota and entitlement fields in `agent models --json`) overlaps 946, 954 and 955; say
where a ticket is covered by 1122's plan.

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1135.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1135 | | | pending |
