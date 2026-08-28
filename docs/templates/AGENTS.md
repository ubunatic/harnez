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

- Subagent handoff must not block the main chat. When the user asks to hand work to a subagent,
  spawn/delegate the task and remain responsive as the host orchestrator; do not immediately wait
  on the child agent unless the user explicitly asks you to wait or the next user-visible
  integration step truly cannot proceed without the result.
- Do not spawn subagents with git worktree isolation unless the user explicitly requests it.
  Sequential/consecutive ticket work should run directly on the currently checked-out branch —
  worktrees have their own failure modes (e.g. branching from a stale base, or being unable to
  see a prior step's still-uncommitted changes) and add reconciliation overhead that isn't needed
  for normal one-after-another dev work.

