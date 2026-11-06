# 1140 — Jules backlog batch: epics (merge/close assessment)

**Status**: Closed — Batch applied in the 2026-11-06 Jules backlog cleanup; per-ticket results in this ticket's Jules runs table and docs/studies/2026-11-06-jules-backlog-cleanup.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 825, 908, 909, 921, 940, 981, 982, 1050

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 825 Epic: harnez serve, durable any-client agent server
- 908 Epic: agent sessions, liveness, compaction and shell awareness
- 909 Epic: system robustness, privileged prompts, swap and kill diagnostics
- 921 Epic: harnez server as monitor for local and remote AI hosts
- 940 Epic: recommend neus in skills and hooks, gated and with fallbacks
- 981 Epic: workflow skills and skill installation
- 982 Epic: test and release hygiene
- 1050 Epic: Pi background task experience and reliability

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1140.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1140 | 4664725194211660920 | github #104 (closed) | done 16m: 8 epics keep. 6/6 refs hold; wrong claim on 981 (bug-hunt skill exists, docs/commands/bug-hunt.md). No ticket changes |
