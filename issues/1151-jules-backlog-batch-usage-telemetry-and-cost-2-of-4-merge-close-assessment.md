# 1151 — Jules backlog batch: usage, telemetry and cost (2 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 385, 435, 453, 492, 495, 534, 535, 542, 564, 587

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 385 Tokens command to count tokens in files and directories
- 435 Native Subagent Replacement, Harnez Agent Dispatch Interception, and A/B Telemetry Switch
- 453 Clarify Opus:low versus Astra:low cost guidance in sprint documentation
- 492 Persist component selection in user-local config and move local config loader out of internal/usage
- 495 Shared token capture package with stable API and spec schemas
- 534 Capture agent session tokens continuously, not only at session end
- 535 Review agent-collector lifecycle automation: install, auto-start, keep current
- 542 Reduce idle CPU of harnez usage --compact --watch (~8% of a core)
- 564 Capture Useful Session Status-Line Usage Data
- 587 MCP server tool calls block synchronously and exceed client timeout without early detachment guidance

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1151.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1151 | 6799427426291656610 | pending | running (started 2026-11-06 01:23, audited 2cf3be4d) |
