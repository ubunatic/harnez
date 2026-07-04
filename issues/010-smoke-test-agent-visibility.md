# Smoke-test that agents can see installed skills and commands

**Status:** Open — blocked on wayreel#11 (V step)

**Severity:** Low — correctness gap, not a data-loss bug

## Problem

The integration test and smoke script verify that claudeconfig *writes* the
right files to the right paths. They do not verify that the target agents
(agy, codex, claude) actually *see* those files — i.e. that the install
paths match what each agent reads at startup and surfaces in its TUI.

A path regression (wrong target dir, wrong filename convention, wrong
directory structure) would pass all current tests but silently break agent
discovery.

## Plan

Use wayreel reels with `V contains=` steps to drive each agent's TUI,
trigger its skill/command listing, and assert expected names appear.

Draft reels are already in `reels/`:
- `reels/smoke-agy.reel` — agy `/skills` → verifies `evergreen`, `domain-modeling`
- `reels/smoke-codex.reel` — codex `/skills` → same checks
- `reels/smoke-claude.reel` — claude `/usage` → verifies installed commands

These reels are spec-complete but cannot run yet: the `V` step is not
implemented in wayreel (tracked in wayreel issue #11).

## What needs to happen

1. wayreel#11 lands (`V contains=` step, shell tee-log capture).
2. Confirm exact slash command for each agent that lists skills/commands
   and produces stdout (needed for the tee approach to work); update reels.
3. Add a `make smoke-agents` target (or extend `scripts/smoke-test.sh`) that
   runs the three reels via `wayreel play` and fails if any `V` check fails.
4. Confirm `smoke-claude.reel` — `/usage` is a UI command that may not print
   to stdout; may need `claudeconfig status` piped inside the TUI instead.

## Related

- wayreel#11 — V (verify) step implementation
- issue#007 — general test coverage gaps
