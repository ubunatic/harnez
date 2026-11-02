# 1097 — Analyze T3 Code (t3.codes) and absorb useful ideas into harnez

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1094 (AutoSwarm research, same method), 302 (research of other agent harnesses), 774 (TUI harness research)

---

/goal Analyze T3 Code (https://t3.codes/, source at github.com/pingdotgg/t3code, MIT) and write a findings note in docs/studies/. Done when the note lists each idea with keep or skip and a reason, and every keep has a follow-up ticket number. Stop and report if the source is unavailable or the licence forbids reuse of ideas. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

T3 Code is a free, MIT-licensed GUI that drives agent CLIs the user already has installed (Codex, Claude Code, Cursor, OpenCode and others). It has worktree support, diff review, and one branch per agent thread. Several of these overlap with harnez's agent sessions, worktree and review work, and its GUI and mobile access overlap with the TUI direction in 774. We want to learn what it does well and take over the ideas that fit harnez, without copying code.

## 2. Technical Specification / Findings

- Initial web search only (2026-11-02). Secondary sources disagree on the supported agent list and on the repository location (`t3-oss/t3-code` or `pingdotgg/t3code`). Verify against the repository before relying on either.
- Status is alpha; one listing shows v0.0.40. Launch date unverified.
- Uncertainties for the study: how it isolates per-thread worktrees, how it reads provider state, and whether its web/desktop server model fits harnez's local-first design.

## 3. Implementation & Verification Plan

- Read the README, docs and the main server and worktree code in the repository. Record the licence and the version studied.
- Write `docs/studies/` note with one row per idea: idea, source file, keep or skip, reason.
- File one ticket per kept idea, link it from this ticket under Related, and list the numbers in the study.
- No code changes under this ticket. Verification: the study exists and `harnez issues lint` passes.
