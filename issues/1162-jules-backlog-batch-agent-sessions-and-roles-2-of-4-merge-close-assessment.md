# 1162 — Jules backlog batch: agent sessions and roles (2 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 468, 502, 510, 538, 548, 549, 566, 611, 612, 623

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 468 Compact `make test-q1` output to summary and failing tests, growing `harnez distill`
- 502 Strengthen AGENTS.md documentation and constant-uniqueness guidance
- 510 Add a web-only researcher role for harnez agent
- 538 harnez agent: report leftover processes at turn end, reap only stopped ones
- 548 Require direct harnez agent commands
- 549 Keep agent Bash calls short and manage prompt files
- 566 harnez bench: optional isolated agent config per run
- 611 Host orchestrator polling loop during synchronous agent tasks
- 612 Native peer discovery lacks session registry and conversation ID lookup
- 623 Short peer-addressable names for interactive agent sessions

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1162.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1162 | 18022529935140632501 | pending | running (started 2026-11-06 02:16, audited 9e7a8f16) |
