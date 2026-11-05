# 1130 — Jules backlog batch: lean-sprint skill tickets (merge/close assessment)

**Status**: Closed — Batch applied, all kept, reports/jules/1130.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 545, 670, 699, 705, 832, 834, 835, 845

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 545 Lean-sprint guardrails: history preflight, CPU check for loop/wait changes, agent start --dry-run, host-only ticket close
- 670 lean-sprint handoff: every prohibition names the allowed alternative; API mappings as full tables
- 699 Rate lean-sprint sessions before teardown
- 705 lean-sprint: mark ticket In Progress at dispatch
- 832 lean-sprint: handoff lists old vs new lines for output changes
- 834 lean-sprint: developer runs the real deploy or host command once before committing
- 835 lean-sprint: allow agy:argon:low as architect tier
- 845 lean-sprint: absolute paths for out-of-repo files in the developer handoff

Note: the lean-sprint skill source may live outside `skills/` in this repo; check where it is before calling a
change missing.

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1130.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1130 | 16131954881067761762 | GH #88 (closed) | done: all 8 kept; refs in `docs/commands/lean-sprint.md` held, each gap still open |
