# 894 — Epic: lean-sprint sweep of P1/P2 and value-add tickets (vendor-mixed workers), evergreen and wrap

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 893, 889, 866, 871, 887, 879, 851, 852, 878, 880; docs/commands/lean-sprint.md

---

/goal Complete the tickets below with `/lean-sprint` runs, one sprint per row group, then run `/evergreen` and
`/wrap`; each ticket closes only when its own acceptance holds, a shortfall gets its own ticket. Stop and report
when a worker loops or is stuck, a blocker cannot be solved, or a ticket needs an owner decision.

## Workers (vendor-mixed)

Developer `sol:med` or `haiku:high`; searcher `haiku:low` or `luna:low`; architect `sol:med` or `opus:med`.
Within a sprint, mix vendors (Codex/Claude) across searcher, architect and developer. Host reviews haiku developer
work with extra care (weaker than sol on terminal coding).

## Tasks

| # | Sprint | Ticket | What | Workers (searcher → architect → developer) | Status |
|---|---|---|---|---|---|
| 1 | 1 easy | 893 | Haiku as searcher and simple-task developer in spec and skills | luna:low → – → haiku:high | Open |
| 2 | 1 easy | 889 | Docs: canary and modularization guidelines for in-tree sandbox copy fixtures | same | Open |
| 3 | 1 easy | 866 | `release` refuses while breaking-changes table has Unreleased rows | same | Open |
| 4 | 1 easy | 871 | Pre-flight disk space check before heavy downloads and builds | same | Open |
| 5 | 2 medium | 887 | Signal-kill diagnostics: quick cause hints, rotating kill log | haiku:low → – → sol:med | Open |
| 6 | 3 complex | 879 M2 | One-shot `harnez usage` recovers remote load | haiku:low → sol:med → haiku:high | Owner decision pending |
| 7 | 4 skills | 851 | bug-hunt skill | luna:low → opus:med → sol:med | Open |
| 8 | 4 skills | 852 | tui-check skill | same | Open |
| 9 | 4 skills | 878 | retro skill | same | Open |
| 10 | 4 skills | 880 | jam skill (stop and ask if it needs new harnez commands) | same | Open |
| 11 | end | — | `/evergreen`: curate learnings into docs/, sync issues and indexes | host | Open |
| 12 | end | — | `/wrap`: stop helpers, sync tracker, commit, final report | host | Open |

Sizes are checked by each sprint's searcher; a larger ticket moves into its own sprint.

## Open decisions (owner)

- Architects at `:med`: decided "change skill" (2026-10-24); lean-sprint now allows `opus:med` and `sol:med`.
- 879 M2: owner answered "ok" to the recommendation, read as option 1; host confirms before Sprint 3. (1) collector fetches remote load in the background (promote 161), (2) detached refresh per one-shot
  run, or skip.

## Out of scope

864, 869, 870 (need design), 865, 885 (system changes).
