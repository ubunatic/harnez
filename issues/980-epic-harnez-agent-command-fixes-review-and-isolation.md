# 980 — Epic: harnez agent command fixes, review and isolation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 959, 965, 971, 958, 962, 961

---

/goal Complete the tickets below with `/lean-sprint` runs; each closes only when its own acceptance holds, a
shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a ticket needs
an owner decision. Circuit breaker: on no progress, clear the goal and write a failure report here.

Bugs first, then features. Web fanout only for 962 (isolated mode: existing sandboxing for agent CLIs).

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 959 | P2 | Role inference turns a fix prompt into a read-only reviewer (bug) | Open |
| 2 | 965 | P2 | Issue close records stale inherited worker model and session (bug) | Open |
| 3 | 971 | P2 | `agent start --stream` stats drop the final reply when already printed (bug) | Open |
| 4 | 958 | P3 | `agent review` subcommand for second-vendor diff audits | Open |
| 5 | 962 | P2 | Isolated mode: inference only, no files, network, skills or tools | Open |
| 6 | 961 | P2 | Agents write scratch files to `.harnez/scratch`; init creates and ignores it | Open |
| 7 | — | — | `/evergreen`, `/wrap` | — |

Sprint groups and worker tiers are set by `/epic 980` refinement.
