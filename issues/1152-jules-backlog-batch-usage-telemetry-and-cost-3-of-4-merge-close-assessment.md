# 1152 — Jules backlog batch: usage, telemetry and cost (3 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 607, 648, 662, 663, 677, 690, 704, 720, 725, 739

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 607 Adopt Loom view for usage --compact --watch behind a flag (strangler, from loom 103)
- 648 Show Claude 5h and weekly used percent in the default usage view
- 662 30-minute dev budget: keep the default background limit, guide the host when a turn dies
- 663 harnez usage --check: machine-checkable compact acceptance
- 677 Show rate-fetch status consistently in usage watch TUI
- 690 Usage table display width and empty state
- 704 Quota-1: host verification rerun runs make test, not test-q1
- 720 Split telemetry analytics and datavis out of harnez into a new telemetry project
- 725 Record user-launched Pi sessions in harnez telemetry via a Pi extension hook
- 739 Rules header lists Quota.md when quota-1 is not enabled

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1152.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
