# 465 — Consider adopting loom's lean-sprint field notes into docs/practices AgenticLoop

**Status**: Open
**Priority**: P3
**Severity**: Minor
**Category**: Docs / Practice
**Related**: 288, 356, `docs/practices/AgenticLoop.md`, `AgenticLoop.lite.md`

## Goal

Decide whether, and in which form, the lean-sprint lessons from loom belong in harnez's managed
`AgenticLoop` practice docs (full and lite variants), then adopt what fits.

## Source

loom `docs/AgenticLoop.md` section 3, "Lean sprints with cheap developer agents (2026-09 field
notes)", and `docs/studies/2026-09-21-lean-sprint-session-report.md` (12 tickets, Haiku, luna:low,
sol:low, Sonnet). Read them in the loom repo (`../loom`) before deciding.

## Candidate items

- Plan first: demand a rough plan from the developer before it codes.
- Escalation ladder (Haiku, luna:low, sol:low, Sonnet, Opus) with step-level handback down.
- Review for fakes: stubs, vacuous tests, tests pinning a bug, evidence that proves nothing.
- Evidence convention: code-generated `.ansi` frames in `docs/progress/<ticket>/`, own test only.
- Mechanics: `harnez agent` is synchronous (run as background task); index.lock (see 464).
- Goal Stop-hook trap (see 466).
- Collection ticket for human-only checks, leftovers as tickets.

Note: the working tree currently has uncommitted edits to `docs/AgenticLoop.md`; check whether
they overlap before merging.
