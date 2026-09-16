Adhere to the following conventions.

<!-- harnez:begin Local Overlays -->
- Local ephemeral overrides: @AGENTS.local.md
<!-- harnez:end Local Overlays -->

<!-- harnez:begin Project Summary -->
<!-- harnez:end Project Summary -->

<!-- harnez:begin Harnez Managed Conventions -->
## Harnez Managed Conventions

Managed by harnez — local edits here are overwritten on the next `harnez init`.
Put project-specific rules outside this block.

### Editing Discipline
- Prefer structured patch tools (`apply_patch`) or whole-block replacements over
  narrow string substitution edits.
- When making multi-line edits, ensure sufficient surrounding context lines to
  avoid ambiguous pattern matches.

### Issue Tracker Discovery (harnez find)
Applies when this project has an `issues/` tracker. To search existing issues,
compute the next ticket number, or allocate one, use `harnez find` / `harnez issues`
instead of `ls issues/`, `find`, or raw grep:
- `harnez find -d <repo> issues -a status:open` — list active open issues
- `harnez find -d <repo> issues "<query>"` — fuzzy search across titles and body text
- `harnez find -d <repo> issues next` — report the next free ticket number (read-only)
- `harnez issues new -d <repo> "<title>"` — atomically reserve that number and create
  a placeholder ticket file; write the ticket to the printed path
- `harnez issues <verb> -d <repo> <n> [reason]` — change a ticket's status, resync
  `issues/README.md`, and commit, in one call
- `harnez index -d <repo>` — update `issues/README.md` after filing or updating tickets
- Commit documentation and `issues/*.md` changes immediately; don't batch them behind
  pending code work.

### Agentic Loop Invariants
Where `@docs/AgenticLoop.md` is present in this project, follow it rather than
restating it here — in particular Invariant 1 (Parallel Read, Sequential Write:
one writer per workspace), Invariant 3 (Zero Zombie Guarantee: track and terminate
every background task and subagent), Invariant 6 (Context Discipline: no whole-file
reads of AGENTS.md/CLAUDE.md — grep or range-bounded reads), and Invariant 7
(Media & Demo Verification Gate: explicit user confirmation before publishing
recordings or screenshots).
<!-- harnez:end Harnez Managed Conventions -->

## Development Scripts

Run from project root.

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

