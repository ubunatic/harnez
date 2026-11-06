---
kind: sprint
date: 2026-11-06
tickets: [1175]
---
# Lean sprint 1175: usage watch debug frame loses lines after a collector refresh

## Context

- Ticket 1175: `TestBuildWatchFrameAtDebugOverlayUsesCollectorCadence` fails on main
  (`before=89x24/16 lines after=89x24/13 lines`); it also failed on 74e2ca78~1, so it predates the
  Jules cleanup session that found it.
- Host: Claude Opus 5.5 in decide mode. Owner asked for "available models": Codex had 5% of its 5h
  window used, Claude 73%, so helpers run on Codex.
- Git mode `team`: work on `sprint/1175`, AGit PR to main.

## Outcome

- Searcher `search-1175` (`--role advisor --model cheap` = codex:luna:low): running.

## What Went Wrong

None yet.

## Learnings

To be filled.

## Follow-Up Tickets

None yet.
