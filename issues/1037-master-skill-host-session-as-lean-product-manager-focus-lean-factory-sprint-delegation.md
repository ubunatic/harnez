# 1037 — /master skill: host session as lean product manager (focus + lean/factory sprint delegation)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[145-orchestrator-session-skill-and-command]] (overlaps; supersede or merge), [[998]] (`/focus`), `commands/lean-sprint.md`, `commands/factory-sprint.md`, [[1035]] (`/goal` hook vs invoke-only sprint skills)

---

## Goal

/goal Ship a `/master` skill that puts the host session in a lasting product-manager
mode: grounded in the project the way `/focus` does it, and running every piece of work
as a `/lean-sprint` or `/factory-sprint` delegation to token-efficient subagents.
Verify it in one live session. Stop and report when blocked on a user decision or a
denied permission. On a loop, a hard blocker, or no progress, clear the goal and write
a failure report here.

## 1. Problem & Motivation

The owner sets up a session by hand: "see /lean-sprint and /factory-sprint, these are
the modes we will work in", then `/focus`, then picks tickets. The role (never code, delegate
everything, stay responsive, think as the product owner) is spread across those three
skills and gets lost in long sessions. The orchestrator-mode idea in 145 covers
delegation only, not the product-manager part.

## 2. Proposal

- `/master [lean|factory] [repo...]` turns the mode on for the rest of the session:
  - runs `/focus` on start: the product, the top 2-3 priorities, and a prompt for the goal;
  - takes on the Host Orchestrator invariants of the chosen sprint skill for *all*
    work: zero coding, diff-first review, no exploratory digging;
  - sends each request through the cheapest suitable path: tickets go to
    `/issue`, small fixes to a lean sprint with a cheap developer, specified multi-milestone
    work to a factory sprint, and research to a Searcher or Architect;
  - stays responsive and reports job ids, compacts reused helpers (1036), and tears down helpers.
- The skill should stay thin: reference the sprint and focus skills, never copy them.

## 3. Open Questions

- Default sprint mode: should the host pick lean or factory per ticket, or keep one fixed for the session?
- `factory-sprint` sets `disable-model-invocation`. Can `/master` route to it, or must
  the user type it? This ties into 1035.
- Multiple repos ("the projects"): should one `/master` session manage several repos?
- How it relates to 145: close 145 as superseded, or build `/master` on top of it.

## 4. Verification

- The skill installs and is listed for every harness that `/focus` supports.
- In a live session, `/master lean` gives a focus summary, then a request is delegated
  without any host edits to source files, and the session ends with no helpers left
  (`harnez agent list` is empty).
