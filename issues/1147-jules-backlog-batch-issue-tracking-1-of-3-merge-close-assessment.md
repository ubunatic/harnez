# 1147 — Jules backlog batch: issue tracking (1 of 3) (merge/close assessment)

**Status**: Closed — Batch applied in the 2026-11-06 Jules backlog cleanup; per-ticket results in this ticket's Jules runs table and docs/studies/2026-11-06-jules-backlog-cleanup.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 279, 283, 293, 340, 451, 471, 615, 616, 693, 696

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 279 Avoid persistent issues README lock sidecar in working trees
- 283 Reassess mandatory fresh-subagent issue filing against direct host filing
- 293 Make roadmap synthesis recoverable across quota interruptions
- 340 Support label, project, and category query filters in harnez find issues
- 451 Roadmap skill should default to low-cost models when unspecified
- 471 Root docs copies drift from copyable sources (Bash, Make, IssueTracking, Spec); reconcile with source-wins rule
- 615 Auto-index issues README on harnez issues new
- 616 Clarify supported issue verbs and rate command protocol in managed rules
- 693 harnez index: say when it replaces a hand-edited row and point at harnez:topic
- 696 harnez read: support direct ticket number targets and error hints for malformed -L line ranges

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1147.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1147 | 15985040144180438253 | github #109 (closed) | discarded (>25m, one resumed wait): 2/5 refs hold (index.go:502 is :44, issue.md:126 is :106, init.go:49 is :24). Closed 615 on host verification (live: stdout `NNN<TAB>path`, stderr `README refreshed`, README lists the new ticket). 451 not obsolete: filed 2026-09-20, after roadmap.md's strong-model text (dd78359d, 2026-09-18); still valid |
