# 1133 — Jules backlog batch: telemetry fields and cost data (merge/close assessment)

**Status**: Closed — Batch applied, reports/jules/1133.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 421, 445, 446, 458, 461, 463, 487, 609

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 421 Extend telemetry for Harnez commands and feature usage analytics
- 445 Add counterfactual API cost rate cards to spec and expose in telemetry reporting and database
- 446 Collect reported cost fields in agent responses and persist to telemetry database
- 458 Move remaining telemetry SQL into `spec/` (127 follow-up)
- 461 Add `model` column to `tool_calls` for per-model call analytics (457 finding)
- 463 Track escalated agent starts and resumes with a reason
- 487 Telemetry: record agent role and parent for worker calls, attribute nested harnez subcommands
- 609 Keep finder query text in local telemetry (opt-in)

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1133.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1133 | 6769701841380360372 | GH #90 (closed) | done: all kept. Rejected closes: 421 (`cli_invocations` from 326, `spec/telemetry.yaml:105`, lacks 421's agent-vs-user feature analytics); 445, 446 (`compaction_economics` is neither rate cards nor reported cost). 458, 461, 463, 487, 609 kept |
