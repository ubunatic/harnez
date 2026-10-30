# 1037 — /pm skill: host session as lean product manager (focus + lean/factory sprint delegation)

**Status**: Closed — M1 2ff9264f (auto-trigger focus/lean-sprint/factory-sprint), M2 5026b0ac (1036 compaction rule), M3 53a39c82 (/pm skill, explicit-only). Live check: /pm in a fresh host session loaded, ran focus, and reported the sprint mode; the owner confirmed it works; harnez agent list empty. make lint: 213 pre-existing issues, unchanged. test-q1: only the pre-existing TestGoldenFontAssets failure.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[145-orchestrator-session-skill-and-command]] (overlaps; supersede or merge), [[998]] (`/focus`), `commands/lean-sprint.md`, `commands/factory-sprint.md`, [[1035]] (`/goal` hook vs invoke-only sprint skills)

---

## Goal

/goal Ship a `/pm` skill that puts the host session in a lasting product-manager
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

- `/pm [lean|factory] [repo...]` turns the mode on for the rest of the session:
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
- `factory-sprint` sets `disable-model-invocation`. Can `/pm` route to it, or must
  the user type it? This ties into 1035.
- Multiple repos ("the projects"): should one `/pm` session manage several repos?
- How it relates to 145: close 145 as superseded, or build `/pm` on top of it.

## 4. Verification

- The skill installs and is listed for every harness that `/focus` supports.
- In a live session, `/pm lean` gives a focus summary, then a request is delegated
  without any host edits to source files, and the session ends with no helpers left
  (`harnez agent list` is empty).

## Decision

- 2026-10-30, owner: the skill is named `/pm` (product manager). `/master` read as a git branch name or as master/slave.
- 2026-10-30, owner: `focus`, `lean-sprint` and `factory-sprint` become auto-triggerable; that answers the
  disable-model-invocation question in §3, so `/pm` can route to factory-sprint.
- 2026-10-30, owner: one combined branch `sprint/1036-1037` and one PR, replacing PRs #11 and #12.
- Host: sprint mode is chosen per ticket (factory for specified multi-milestone work, lean otherwise).
  Each repo gets its own `/pm` session, so multi-repo management is out of scope. Ticket 145 closes as superseded.
- Host: the threshold-based compaction trigger and the `resume` hint from 1036 are parked in a follow-up ticket;
  1036 delivers the documented compaction step only.
- Host (plan review): `/pm` itself stays explicit-only, because a mode change that lasts the whole session should not fire on its own;
  the `[repo...]` argument is dropped. The cost is accepted: the three auto-trigger descriptions load into every Claude session.
- Host (plan review): 1036 closes once the rule has landed. Its live two-milestone Pre/Post verification moves to the
  parked follow-up ticket. The developer does not close 1037; the host closes it after checking the installed
  copies and doing a live `/pm` check.

## Milestones

- **M1 (auto-trigger)**: drop `user-invocable-only` for `focus`, `lean-sprint`, `factory-sprint`
  (`config.yaml` ~167-169) and rewrite their descriptions so the model knows when to pick them
  (remove "Only on explicit /focus"). Installed copies for every harness must lose the invoke-only
  marker (`disable-model-invocation` for Claude).
- **M2 (1036 searcher compaction)**: in the factory-sprint skill (Phase 1) and the lean-sprint skill (§1), compact a
  reused Searcher with `HTO=0 harnez agent compact --name <searcher>` between milestones or after
  a large search round. Close 1036.
- **M3 (`/pm` skill)**: a thin new skill per §2 (runs focus first, then follows the orchestrator invariants and
  routes each request), registered like `focus`; close 145 as superseded; close 1037.
