# 1141 — Jules backlog batch: Jules (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 1073, 1074, 1089, 1091, 1093

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 1073 Jules setup script installs harnez, loom and the tools for working the harnez way
- 1074 Sync main to GitHub from this machine and before Jules, without CI, force or pruning
- 1089 Jules start: refused run keeps its name, vague refusal, unexplained compare failure
- 1091 Jules driver: sync fetch can miss the compared commit, and cut-off responses give a vague error
- 1093 Failed Jules start leaves a session record that blocks the retry name

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1141.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1141 | 12594687266649708746 | pending | running (started 2026-11-06 00:45, audited 9f7dcf78) |
