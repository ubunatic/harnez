# 1035 — Goal hook cannot wait for invoke-only sprint skills, so agents bypass the requested sprint

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: `docs/commands/epic.md` (frontmatter `disable-model-invocation: true`), lean-sprint, factory-sprint; neus session 2026-10-29 (neus issue 111 M2)

---

## 1. Problem & Motivation

`/epic`, `/lean-sprint` and `/factory-sprint` are invoke-only skills (`disable-model-invocation: true`).
In a neus session, the owner set a `/goal` and said "use /lean-sprint or /factory-sprint as needed
to run /epic". The agent's Skill calls were refused, so it asked the owner to type `/epic`. Before
the owner could answer, the goal stop hook demanded progress again, and the agent built the feature
directly. The owner's requested workflow (sprint with review gates) was silently skipped. The
result was fine (neus 111 M2), but the process the owner asked for did not run.

## 2. Technical Specification / Findings

Two mechanisms conflict: invoke-only skills require the user to type the command, while the goal
hook treats waiting for the user as no progress. Neither the goal text nor the hook offers a "waiting
for a user-typed skill" state. Candidate fixes, to be decided:

- The goal hook recognizes a pending user-invoked skill (or a "blocked on user" marker) and pauses
  instead of pushing.
- Sprint skills allow model invocation when an active `/goal` names them explicitly.
- The goal guidance (Harnez Agent section, "offer `/goal ... or stop and report when blocked`")
  tells the agent to stop and ask for the skill rather than proceed when a required skill is
  invoke-only.

## 3. Implementation & Verification Plan

`/goal Make a /goal that requires an invoke-only sprint skill either run that skill or stop and
wait for the owner, never silently bypass it; or stop and report when blocked on a user decision or
denied permission. On a loop, hard blocker, or no progress, clear the goal and write a failure
report with the reason.`

Check live hook and skill code first. Verify with a goal that names `/lean-sprint`: the agent
either runs the sprint or stops with one clear request to the owner, and the hook does not force
direct implementation.
