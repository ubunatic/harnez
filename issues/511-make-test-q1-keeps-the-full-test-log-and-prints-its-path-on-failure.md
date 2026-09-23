# 511 — make test-q1 keeps the full test log and prints its path on failure

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `Makefile` (`test-q1`), `harnez exec --quota-1`, `docs/Testing.md` (Quota-1 pitfall), [[509-session-tip-leaks-into-cmd-harnez-tests-run-inside-an-agent-session]]

## Problem

Under Quota-1 an agent gets one test run per code change. On 2026-09-23 the agent piped
`make test-q1` into `tail`, which cut off the name of the failing test. Quota-1 blocked a
rerun until the code changed, so the run was wasted. `docs/Testing.md` now documents the
pitfall, but only an agent that reads it benefits.

## /goal

Every `make test-q1` (or `harnez exec --quota-1`) run writes its full output to a
known log file, and on failure its last line names that file and the failing tests
(`--- FAIL` lines). An agent that sees only the tail still knows what failed and where the
details are.

## Open questions

- Log location (repo-local ignored dir vs `~/.harnez`) and retention.
- Whether this belongs in `harnez exec --quota-1` (all projects) or only the Makefile.

## Done when

- A failing run's final lines include the log path and the failing test names.
- A test covers the summary on a synthetic failing test run.
