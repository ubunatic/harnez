# 1169 — Jules backlog batch: miscellaneous (1 of 2) (merge/close assessment)

**Status**: Closed — Batch applied in the 2026-11-06 Jules backlog cleanup; per-ticket results in this ticket's Jules runs table and docs/studies/2026-11-06-jules-backlog-cleanup.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 177, 178, 225, 295, 384, 569, 618, 679, 732

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 177 Lean, Scoped Post-Edit Build Check for Control-Flow-Reshaping Edits
- 178 Distill: Opt-In Smart Mode with Classifier-Driven Error Pattern Preservation
- 225 Local SLM Batch Classifier: Reliability & Accuracy Follow-Up
- 295 Splash: Actionable In-Place Operation Status With Duration and Failure Reasons
- 384 harnez assess directory validation and -d/--dir flag support
- 569 harnez bench run: stderr preamble and live progress, table on stdout
- 618 Prevent stale embedded specs when Go packages are separated from canonical specs
- 679 Split oversized Go files into cohesive packages with clean interfaces
- 732 harnez status warns when its own data grows past a size limit

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1169.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1169c | 16654191772771631779 | github #127 (closed) | done (>25m, one resumed wait): 6/6 refs hold; no closes proposed. All kept |
| batch audit | jules-batch-1169, jules-batch-1169b | - | none | start refused: HTTP 400 Precondition check failed (11 Jules sessions incl. stuck 1152, 1157); restarted as 1169c |
