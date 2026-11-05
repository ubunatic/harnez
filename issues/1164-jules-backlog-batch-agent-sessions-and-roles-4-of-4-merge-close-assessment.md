# 1164 — Jules backlog batch: agent sessions and roles (4 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 931, 963, 967, 984, 1033, 1038, 1044, 1047, 1049

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 931 remote: ssh-agent socket under PrivateTmp and remote-load retry durations in spec
- 963 Teach agents to ask where rules belong before editing AGENTS.md
- 967 Verify harnez applies all concepts from the agent harness video (XJuLcDxtTz0)
- 984 purpose aliases: dev:<any> guesses the nearest trait, like --role guessing
- 1033 Tool-call limit per agent turn, enforced by harnez, not by the prompt
- 1038 Threshold-based compaction trigger and resume hint for reused searchers (follow-up to 1036)
- 1044 Aggregate Pi background task completions without session summaries
- 1047 Queue Pi task completions safely during compaction
- 1049 Guard Pi session reload while background tasks are running

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1164.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
