# 1153 — Jules backlog batch: usage, telemetry and cost (4 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 761, 767, 917, 923, 927, 944, 989, 1028

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 761 Harness refinements: Quota-1 test logging guidance, block lexer patterns, and subagent prompt safeguards
- 767 usage: show total remote traffic in Remote Load title
- 917 usage: server-driven boxes and items from spec (label, bar, value, semantic color)
- 923 usage: btop-style coloured hotkey char in box titles via loom
- 927 lint: gokay check fails on usagestore store_test.go errcheck and spec schema conformance
- 944 quota-1: count targeted and coverage go test runs toward the one-run budget
- 989 make install leaves a stale telemetry collector running; new record kinds wait in the spool
- 1028 Add [S] Sessions box card to harnez usage --watch (turns, tokens, compactions)

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1153.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
