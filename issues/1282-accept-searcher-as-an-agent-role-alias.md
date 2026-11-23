# 1282 — Accept searcher as an agent role alias

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [taia #383](https://codeberg.org/harnez/taia/issues/383), #692, #893

---

## Problem
Lean-sprint uses the term searcher, but `harnez agent` reportedly does not accept `searcher` as a role alias. Existing #692 implemented general role aliases; #893 describes searcher as an advisor task.

## Goal
Accept `searcher` as an appropriate role alias and verify role resolution and relevant guidance agree. Check live code and recent history first. Stop and report on a user decision or denied permission; on a loop, hard blocker or no progress, clear the goal and record a failure report and resume point.

## Decision
Include `cmd/harnez/testdata/help.golden` in the write set: CLI help is generated from the alias spec and its existing golden test requires the updated output. Preserve unrelated changes and assertions.

## Blocker / Resume point
The developer implemented the alias and tests in four uncommitted files. `make test-q1` exited 2 before tests: bubblewrap could not set up the UID map (`Permission denied`) in this sandbox. No quota bypass was attempted. Resume with the existing edits in a host environment that permits the quota sandbox; run the required quota suite, review the diff and assertions, install and live-check the CLI, then commit and close only after verification. The runner-mode shared installation is also host-only.
