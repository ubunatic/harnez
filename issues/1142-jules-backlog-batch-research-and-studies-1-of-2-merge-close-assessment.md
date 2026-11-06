# 1142 — Jules backlog batch: research and studies (1 of 2) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 151, 294, 302, 305, 332, 350, 465, 472, 473, 516

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 151 Research: on-demand doc lookup vs. materialized instructions, and temporary per-repo agent profiles
- 294 Investigate cross-agent post-edit success hooks for the harnez rate pipeline
- 302 Research plugin systems for pi agents, DeepSeek Harness, and Prime Agent self-modification
- 305 Investigate `stats --auto` failure rates disagreeing with observed tool outcomes
- 332 Consider a real HARNEZ_DB_PATH/HARNEZ_STATE_DIR production override
- 350 Consider a `harnez memory` command to list/clear Claude Code auto-memory
- 465 Consider adopting loom's lean-sprint field notes into docs/practices AgenticLoop
- 472 Investigate Dream-RSI-inspired replay and exploration practices for harnez
- 473 Study MiniMax CLI harness efficiency and adopt the best Harnez practices
- 516 Measure agy plan-quota cost on Google Pro

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1142.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1142 | 5623484547887532994 | github #110 (closed) | done (>25m, one resumed wait): 10/10 refs hold. Closed 302 (docs-only deliverable: docs/studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md). Rejected: 332 (Testing.md:25 still 'no override today'), 350 (study says candidate ticket, not now; deinvest lead), 465 (partly adopted: plan-first, escalation, .ansi, stop-hook in AgenticLoop.md; review-for-fakes and index.lock missing), 516 (Models.md:140 agy still unmeasured). 9 kept |
