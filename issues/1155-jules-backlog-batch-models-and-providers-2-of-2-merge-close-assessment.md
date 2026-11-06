# 1155 — Jules backlog batch: models and providers (2 of 2) (merge/close assessment)

**Status**: Closed — Batch applied in the 2026-11-06 Jules backlog cleanup; per-ticket results in this ticket's Jules runs table and docs/studies/2026-11-06-jules-backlog-cleanup.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 755, 762, 776, 851, 872, 1034

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 755 Add Claude Haiku 5.5 to the harnez model list
- 762 Rollup: Review and audit cross-provider messaging and supervised AGY queueing changes
- 776 harnez tui External Harness Forwarding and Subscription-Aware Model Router
- 851 bug-hunt skill: pick P1/P2 bugs, re-verify, close obsolete ones, fix the rest the way forward
- 872 agent turn liveness for silent providers (agy), opt-in, no orchestrator noise
- 1034 Local-model runs: attach all input as files and give tight role guidance, so the agent reads nothing itself

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1155.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1155 | 11638024735076166476 | github #117 (closed) | done (>25m, one resumed wait): 6/6 refs hold. Verified 755 done, close needs 1174 (spec/agent.yaml:174-186: claude-haiku-5-5 with tier, roles, cost). Rest kept |
