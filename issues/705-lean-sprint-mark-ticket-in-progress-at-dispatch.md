# 705 — lean-sprint: mark ticket In Progress at dispatch

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Process
**Related**: `docs/commands/lean-sprint.md`, `docs/OrchestratedAgentFlow.md`, `harnez issues start` (issue 232); loom sprints 263/264 (2026-10-04)

---

/goal The sprint skills set the target ticket to `In Progress` when the first developer is dispatched, so the issue index shows running sprints; stop and report if the user prefers the status to come from `harnez agent start` itself.

## Problem

During loom sprints 263 and 264 the tickets stayed `Open` for hours while a developer worked on them (264 had seven developer turns). The issue index could not tell running work from backlog. `harnez issues start N "<reason>"` already exists but no sprint skill calls it.

## Proposal

- Step 1 of `lean-sprint` (and `sprint`, `reverse-sprint`): after the plan is approved and before the first write turn, run `harnez issues start N "developer <name> on <model>"`.
- Teardown already closes or parks the ticket; a parked/aborted sprint resets it to `Open` with a reason.
- Option (record, don't build yet): `harnez agent start --issue N` does this automatically.

## Acceptance

- Skill text updated in `docs/commands/*.md` and the bundled skill copies.
- One real sprint shows `In Progress` in `issues/README.md` while the developer runs.
