# 910 — Epic: repo setup, tracker, tests and agent guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 907, 897, 864, 855, 867, 353, 898

---

/goal Complete the tickets below with `/lean-sprint` runs; each closes only when its own acceptance holds, a
shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a ticket needs
an owner decision. Circuit breaker: on no progress, clear the goal and write a failure report here.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 907 | P2 | Tests: import instead of copy, sunset covered canaries, container tests | Open |
| 2 | 897 | P2 | neus as optional dependency: `harnez install` verb and server indexer | Open |
| 3 | 864 | P2 | Push consent lasts the session; a push syncs Codeberg and GitHub | Open |
| 4 | 855 | P3 | `harnez init` wizard: user picks which features a repo gets | Open, dependency 854 met |
| 5 | 867 | P3 | `issues close` refuses when no commit outside issues/ names the ticket | Open |
| 6 | 353 | P3 | Claude-only rule: never propose CxxxE.md changes, check docs/AGENTS.md first | Open |
| 7 | 898 | P3 | Recommend `gokay check` before commits in Go repo guidance | Open |

Rows 6–7 are small doc tickets and can share one sprint. Sprint groups and worker tiers are set by `/epic 910`
refinement.
