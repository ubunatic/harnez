# 911 — Epic: skills, first real runs and epic/retro hygiene

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 851, 852, 878, 904, 905

---

/goal Complete the tickets below; skill tickets close only after one real run of the skill meets their
acceptance, a shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a
ticket needs an owner decision. Circuit breaker: on no progress, clear the goal and write a failure report here.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 851 | P2 | bug-hunt skill | Shipped; open until first real run |
| 2 | 852 | P2 | tui-check skill | Shipped; open until first loom-widget run |
| 3 | 904 | P2 | Epic skill evaluation: lessons from Neus Next Wave | Open |
| 4 | 905 | P2 | Epic/retro hygiene: file follow-up issues right after studies and retros | Open; overlaps 904, do together |
| 5 | 878 | P2 | retro skill from compaction summaries and key user messages | Not started; needs `harnez session extract` |

## Refinement (2026-10-24)

Web fanout skipped: internal skill and process docs. Code fanout: haiku:low searcher `e911-search`.

| Group | Ticket | Finding | Developer | Reviewer |
|---|---|---|---|---|
| A | 905, then 904 doc parts | skill sources are `docs/commands/epic.md`, `story.md`, plus `docs/AgenticLoop.md`, `docs/IssueTracking.md`; 905 first, both edit epic.md | luna:high | host diff |
| B | 852 | loom at `../loom`, `.ansi` sketches in `docs/data/` (ColorPicker) | host runs the skill | — |
| C | 851 | no active P1/P2 Bug ticket exists, so a run has no input | — | — |
| D | 878 | `harnez session extract` does not exist; new `internal/transcript` package, command, skill | sol:med | opus reviewer |

Open decisions are in `## Decision`.
