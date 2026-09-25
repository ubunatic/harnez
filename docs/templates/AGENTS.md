Adhere to the following conventions.

<!-- harnez:begin Local Overlays -->
- **Before any work, read `AGENTS.local.md` if it exists** (@AGENTS.local.md). It holds this
  checkout's settings (subagent mode, output mode) and overrides this file where they differ.
<!-- harnez:end Local Overlays -->

<!-- harnez:begin Project Summary -->
<!-- harnez:end Project Summary -->

<!-- harnez:begin Harnez Managed Conventions -->
## Harnez Managed Conventions

Managed by harnez — local edits here are overwritten on the next `harnez init`.
Put project-specific rules outside this block.

### Tool Availability
If `harnez` is not installed or available in PATH, install it via:
```bash
go install ubunatic.com/harnez/cmd/harnez@latest
```

### Always `make install`
After every change to a project that has a `make install` target, run `make install` before
reporting or committing, so the user's installed binary always matches the code. This applies in
every repo to solo developers, host sessions that talk to a human, and orchestrators. Leaf developer
subagents in a sprint skip it unless their skill requires it at the end; the host installs after review.

### Editing Discipline
- Prefer structured patch tools (`apply_patch`) or whole-block replacements over
  narrow string substitution edits.
- When making multi-line edits, ensure sufficient surrounding context lines to
  avoid ambiguous pattern matches.
- **Reading & Context Discipline (Recommended for Large Files)**: Prefer
  `harnez read -L <range>` / `harnez read -n` for medium/large files (>100 lines)
  (`harnez read -I` is paused until issue 543, a memory blow-up, is fixed)
  to preserve token quota and prevent context fatigue. Native reads remain valid
  for targeted inspection; hook-level blocking is conditional on the active
  `reading_discipline.enforce` mode in `~/.harnez/config.yaml` (or
  `HARNEZ_READ_ENFORCE`).

### Issue Tracker Discovery (harnez find)
Applies when this project has an `issues/` tracker. To search existing issues,
compute the next ticket number, or allocate one, use `harnez find` / `harnez issues`
instead of `ls issues/`, `find`, or raw grep:
- `harnez find -d <repo> issues -a status:open` — list active open issues (use `-I` for visual overview PNG card)
- `harnez find -d <repo> issues "<query>"` — fuzzy search across titles and body text (use `-I` for visual overview)
- `harnez issues show -d <repo> <n> -I` — render single issue as styled visual PNG card (inspect via `view_file`)
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
reads of AGENTS.md/CLAUDE.md — grep or range-bounded reads), and Invariant 10
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
