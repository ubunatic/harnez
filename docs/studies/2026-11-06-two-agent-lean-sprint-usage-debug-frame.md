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

- Searcher `search-1175` (`--role advisor --model cheap` = codex:luna:low, 41s, 44k new tokens):
  located the test, the 3 lost lines and the write set; its cause (debug-box budgeting) was wrong.
- Developer `dev-1175` (`--role developer --model tui` = codex:sol:med), plan-only first turn (2m):
  found the real cause, live Load sampling: the test summaries carry no `Load`, so host load changed
  the Load text width and row packing folded the All Usage row (`watch_frame.go:401-410`, `:499-535`).
- M1 (`b41b25af`, stable Load width): regression test with deterministic Load at both extremes,
  collector cadence points and exact-fit height; failed before, passed after. Host live check found
  the Load row clipped (`8% (5...`) at 110 columns with free space.
- M2 (`71931b3c`, spare width for Load): Load's preferred width uses the free columns after
  preceding panels (up to 56), independent of the Load text; new test asserts the full row at 110
  columns. Host live check: `cpu (16 cores) [...] 10% (58°C/21W)` at 110 columns; at 90 columns it
  still clips by design (no spare columns). `go test ./internal/usage` green; developer `make test-q1`
  green.

## What Went Wrong

- **Test read the live machine.** The failing test depended on this host's load, so it failed here
  and not everywhere. Fixed by deterministic Load fixtures in the new tests.
- **Green tests hid a visible regression.** M1 passed every test but clipped real data; only the
  host's live PTY render caught it (skill rule: run the installed binary for user-visible output).

## Learnings

- A cheap Searcher maps files and lines well but can guess the cause wrong; a plan-only developer
  turn on a stronger model corrected it before any code was written.
- For layout fixes, stabilise against the volatile input, then check the widest real reading live.

## Follow-Up Tickets

None. The 90-column clip is the intended narrow-terminal behaviour.
