# 1165 — Jules backlog batch: harness integrations (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 466, 563, 647, 724, 1021, 1046, 1048, 1052

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 466 Detect Claude Stop-hook loops and break them (hooks, claude CLI, last resort logout)
- 563 Meter claude and codex traffic like agy (proxy), incl. hidden helper calls
- 647 Document foreground waiting for long-running Codex tasks
- 724 Codex ignores harnez-installed hooks until the user trusts them, but harnez status reports ok
- 1021 Control Pi background task output at launch and avoid duplicate completion dumps
- 1046 Keep Pi task completion output out of the user chat
- 1048 Background manual Pi commands with Ctrl-B
- 1052 Safely refresh the Pi background task extension independently

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1165.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1165 | 6620699448251933694 | github #125 (closed) | done (>25m, one resumed wait): 4/4 refs hold; no closes proposed. All kept |
