# 908 — Epic: agent sessions, liveness, compaction and shell awareness

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 869, 870, 872, 969

---

/goal Complete the tickets below with `/lean-sprint` runs; each closes only when its own acceptance holds, a
shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a ticket needs
an owner decision. Circuit breaker: on no progress, clear the goal and write a failure report here.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 869 | P2 | agy agents auto-compact at 200k tokens by default, per-session on/off flag | Open, nothing built |
| 2 | 870 | P2 | Tell the orchestrator after each new shell what it has open (count + brief) | Open |
| 3 | 872 | P3 | Opt-in turn liveness for silent providers (agy), no orchestrator noise | Open |
| 4 | 969 | P2 | Per-model compaction limit and percent, re-compact and post-compact fill guards | Draft; spec thresholds and modes delivered in 973 M3 (e9ea64de) |

Sprint groups and worker tiers are set by `/epic 908` refinement.
