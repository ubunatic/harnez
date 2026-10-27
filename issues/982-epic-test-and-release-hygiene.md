# 982 — Epic: test and release hygiene

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 944, 951, 950

---

/goal Complete the tickets below with `/lean-sprint` runs; each closes only when its own acceptance holds, a
shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a ticket needs
an owner decision. Circuit breaker: on no progress, clear the goal and write a failure report here.

Internal fixes; no web fanout.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 951 | P2 | usage catchup test: concurrent map writes from parallel collectors (flaky fatal) | Open |
| 2 | 944 | P2 | quota-1: count targeted and coverage `go test` runs toward the one-run budget | Open |
| 3 | 950 | P2 | Go release docs: vet generated version.go before tagging, lowest go directive for libraries | Open |
| 4 | — | — | `/evergreen`, `/wrap` | — |

Sprint groups and worker tiers are set by `/epic 982` refinement.
