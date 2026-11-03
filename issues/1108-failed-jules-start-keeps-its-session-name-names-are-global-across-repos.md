# 1108 — Failed Jules start keeps its session name; names are global across repos

**Status**: Closed — a failed start with no provider session frees its name; old record renamed <name>-failed-<id8> (releaseFailedStartName, cmd/harnez/agent_run.go; TestFailedStartNameIsReleased). Per-repo name scoping not done: the clash error names the global scope and suggests <name>-2.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## Problem
Reported 2026-11-03 by the voxi, lmcoder and loom sessions.

- A start that fails at create (repo not in `github_projects`, HTTP 400 "Precondition check failed") still records its `--name`; a retry with the same name fails with "already in use".
- Names are global across repos: lmcoder's `jules-audit-275` collided with a loom session.
- Re-auditing a ticket needs a new name even after the old session completed.

Mitigated: the error now says names are global, that a failed start keeps its name, and suggests `<name>-2`.

## Goal
/goal A start whose create call fails leaves no record (or its retry reuses it), and a name clash across repos is avoided or explained; stop and report if the store layout needs an owner decision.

## Done when
- A failed Jules create leaves the name free (test with the fake client).
- Decide: scope names per repo, or keep global with the hint.
