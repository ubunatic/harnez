# 569 — harnez bench run: stderr preamble and live progress, table on stdout

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bench / Tooling
**Related**: [[565-harnez-bench-run-model-and-read-mode-matrix-lite-docs-default]]

## Background

`harnez bench run` prints nothing until the whole matrix is done. With several models the user waits
minutes without knowing what runs, which files the agents get, or what a card looks like.

## Goal

- **stdout**: only the final comparison table (pipeable).
- **stderr**: a preamble before any agent is called, then one line per run as it starts and ends.

## M1 — preamble and progress

- Preamble, once per task × read mode of this invocation, derived from the task spec (no
  hand-written per-task scripts): task name and label, read mode, the fixture file(s) the agent is
  pointed to (path and line count), and the question asked.
- `card` mode: generate the card for the fixture in the preamble (same `harnez read -I` path the
  agent uses), print its PNG path, size and pixel dimensions, so the user can open it. If card
  generation fails, stop before calling any agent.
- Optional `info:` string per task in `tasks.yaml`, printed in the preamble when set.
- Progress per run: `[3/6] agy:flash37:low card read-one-fact ...` on start;
  `pass, 15.6k in, 2 turns, 41s` (or the error) on end.
- `-q` suppresses stderr output.
- Unit tests with fake runners: stdout holds only the table; stderr has the preamble before the
  first run and one start/end pair per run; card preamble failure calls no agent.
- No live model calls needed. One `make test-q1`, commit `feat(bench): ... (issue 569 M1)`.

## M1 delivered (host-committed)

Dev's one q1 run was red; it fixed the causes but did not commit. Host q1 on the fixed tree: green;
host committed and installed. Review notes for a possible M2:

- `benchPreamble` loops `for range task.Fixtures` but lists every file in the staged `docs/` each
  time: tasks with 2+ fixtures print the list twice, and non-fixture docs are listed too. List only
  the task's fixtures, once.
- Card preamble uses its own `readcard` render, not the agent's `harnez read -I` path.
- `-q` skips the preamble, so card-failure-stops-before-agents no longer holds under `-q`.
- Card temp dirs are kept on purpose (user opens them); say so in `docs/Bench.md`.
