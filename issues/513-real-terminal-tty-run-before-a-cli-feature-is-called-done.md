# 513 — Real-terminal (TTY) run before a CLI feature is called done

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Process
**Related**: `docs/Testing.md` ("Tests prove invariants; they do not prove every terminal ... rendering outcome"), `docs/practices/AgenticLoop.md` (copyable source), `docs/commands/sprint.md`, `lean-sprint.md`, `reverse-sprint.md`, [[511-make-test-q1-keeps-the-full-test-log-and-prints-its-path-on-failure]]

## Problem

CLI features are called done when their tests pass, but tests write to buffers,
not a terminal. Terminal-dependent behaviour (TTY detection, colors, column
alignment and display width, pagers, stderr/stdout interleaving, prompts, progress
redraws) is never seen the way a user sees it. Agents make this worse because their
shell tools have no TTY, so even "I ran it" means a non-TTY run.

Example (2026-09-23): the `harnez agent models` table was verified by tests and a
non-TTY run; the first real-terminal run was the user's.

## /goal

A CLI feature is not reported done, and its ticket not closed, until it has been run
once in a real terminal and the result recorded in the ticket or report. This rule
is in the copyable practice docs and the sprint skills' done/review gates, so it
applies in every harnez-managed project.

- The agent does the run itself under a pseudo-terminal (e.g. `script -qc '<cmd>' /dev/null`
  or a `tmux` pane plus `capture-pane`) and quotes the captured output, or it asks the
  user to run the command (`! <cmd>` in Claude Code) and says the check is pending.
- The report says which one happened; "tests pass" alone does not satisfy it.

## Open questions

- Scope: every CLI change, or only output-/interaction-facing ones (flags, tables,
  colors, prompts)?
- Whether a small helper (`harnez exec --tty`?) should make the pty run one command
  and strip escape codes for the transcript.

## Done when

- `docs/practices/AgenticLoop.md` (plus synced root copy), `docs/Testing.md` and the
  three sprint skill sources state the rule and the pty recipe; `harnez apply` installs them.
- A pty run of `harnez agent models` via the recipe is shown to reproduce the TTY
  output (proves the recipe works).
