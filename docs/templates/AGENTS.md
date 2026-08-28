Adhere to the following conventions.

<!-- harnez:begin Local Overlays -->
- Local ephemeral overrides: @AGENTS.local.md
<!-- harnez:end Local Overlays -->

<!-- harnez:begin Project Summary -->
<!-- harnez:end Project Summary -->

## Development Scripts

Run from project root.

## Issue Tracking

- Commit documentation changes and `issues/*.md` changes immediately, don't batch them behind
  pending code work.

## Background Tasks & Process Hygiene

- Do not spawn subagents with git worktree isolation unless the user explicitly requests it.
  Sequential/consecutive ticket work should run directly on the currently checked-out branch —
  worktrees have their own failure modes (e.g. branching from a stale base, or being unable to
  see a prior step's still-uncommitted changes) and add reconciliation overhead that isn't needed
  for normal one-after-another dev work.


