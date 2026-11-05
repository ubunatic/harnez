# 1149 — Jules backlog batch: issue tracking (3 of 3) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 977, 978, 990, 1005, 1015, 1020, 1045, 1083

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 977 sprint-pause pause|unpause skill: park a sprint (notes, ticket state, resumable agents) and restart it
- 978 Groom open issues into backlog with a ranked candidate list
- 990 Automatically reconcile issue indexes when agents file tickets concurrently
- 1005 Backfill historical closed-ticket outcomes flagged by status
- 1015 `issues mv` rewrites references to the moved ticket number
- 1020 Add a read-only topic-scoped backlog value scan skill
- 1045 Avoid generated index drift after fast-forward pulls
- 1083 Triage active backlog from 104 tickets down to the 30-ticket cap

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1149.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1149 | 8197884588804528726 | pending | running (started 2026-11-06 00:45, audited 9f7dcf78) |
