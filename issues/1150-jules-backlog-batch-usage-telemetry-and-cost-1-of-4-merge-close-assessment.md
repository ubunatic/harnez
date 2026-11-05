# 1150 — Jules backlog batch: usage, telemetry and cost (1 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 034, 051, 084, 146, 160, 172, 208, 215, 296, 324

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 034 Hook-Triggered File Seek for Local Token Extraction (AGY & Claude)
- 051 Multi-host remote usage monitoring & interactive host navigation
- 084 Aggregate Quota-Window Box: UX & Feasibility Assessment
- 146 Assess recent subagent activity in compact usage watch
- 160 Feasibility: Extract `usage --watch` Layout/UI/Keyboard Into a Renderer-Agnostic Module
- 172 AGY "Claude/GPT" Row Drops Its Second Window at 100%, Breaking All Usage Grid Alignment
- 208 SQLite Export Format for `harnez usage export`
- 215 LLM Backfill and Reclassification Mode for Telemetry Tool Notes
- 296 Always compute distill savings for stats even when HARNEZ_DISTILL_AUTOPIPE is off
- 324 docs/lang/Go.md canonical-pattern reference to internal/usage is dangling in consumer repos

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1150.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
