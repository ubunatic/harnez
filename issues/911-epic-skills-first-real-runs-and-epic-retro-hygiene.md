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
| 1 | 851 | P2 | bug-hunt skill | Parked: no active P1/P2 Bug ticket to run on |
| 2 | 852 | P2 | tui-check skill | Closed: first run on loom ColorPicker (98badd77) |
| 3 | 904 | P2 | Epic skill evaluation: lessons from Neus Next Wave | Closed (65aac99d); close flags moved to 915 |
| 4 | 905 | P2 | Epic/retro hygiene: file follow-up issues right after studies and retros | Closed (826496f1, 361ef481) |
| 5 | 878 | P2 | retro skill from compaction summaries and key user messages | M1–M3 accepted (21beea8b, a52d63bf, d5bbc634); owner-reviewed retro open |

## Refinement (2026-10-24)

Web fanout skipped: internal skill and process docs. Code fanout: haiku:low searcher `e911-search`.

| Group | Ticket | Finding | Developer | Reviewer |
|---|---|---|---|---|
| A | 905, then 904 doc parts | skill sources are `docs/commands/epic.md`, `story.md`, plus `docs/AgenticLoop.md`, `docs/IssueTracking.md`; 905 first, both edit epic.md | luna:high | host diff |
| B | 852 | loom at `../loom`, `.ansi` sketches in `docs/data/` (ColorPicker) | host runs the skill | — |
| C | 851 | no active P1/P2 Bug ticket exists, so a run has no input | — | — |
| D | 878 | `harnez session extract` does not exist; new `internal/transcript` package, command, skill | sol:med | opus reviewer |

## Decision

Owner said "go" to the host's recommendations (2026-10-24):

- 851: parked until an active P1/P2 Bug ticket exists; the epic does not wait for it.
- 852: the host runs tui-check on loom's ColorPicker widget.
- 905 first, then 904's doc parts (phased DAG, sibling decoupling, delivery typology); 904's
  `issues close --model/--session` flags move to their own ticket.
- 878: `retro` is its own skill, not a `story` mode; Claude transcripts only.
