# 1098 — Analyse Herdr and absorb useful ideas into harnez

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1094 (AutoSwarm research), 1095 (T3 Code analysis), 302 (research of other agent harnesses), 774 (TUI harness research)

---

/goal Analyse Herdr and write a findings note in docs/studies/. Done when the note lists each idea with keep or skip and a reason, and every keep has a follow-up ticket number. Stop and report if the source is unavailable or the licence forbids reuse of ideas. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

Herdr is a terminal workspace manager for AI coding agents, not a desktop GUI app as the request said. It runs inside an existing terminal or tmux, shows which agents are idle, working or blocked, and lets agents control it through a local socket API and CLI (create workspaces, split and zoom panes, spawn helpers, read output). Its agent orchestration overlaps with harnez's agent start, resume and status work, and its pane and status view overlaps with the TUI direction in 774. We want to learn what fits harnez and take over those ideas, without copying code.

## 2. Technical Specification / Findings

- Initial web search only (2026-11-02). Sources name several repositories (`ogulcancelik/herdr` as the likely primary, plus forks such as `maxjendrall/herdr`, `naaive/herdr`, `ttbug/herdr`, `Suor/herdr`). Verify the primary repository and site (herdr.dev) before relying on either.
- Tested agents named in its docs: Claude Code, Codex and OpenCode. A third-party count of 17 agents is unverified.
- Licence: AGPL-3.0-or-later, with commercial licences offered. Ideas may be used; code must not be copied into harnez without a licence decision. Record this in the study.
- Uncertainties: how it detects agent state (idle, working, blocked), and whether its socket API design fits harnez's agent CLI.

## 3. Implementation & Verification Plan

- Read the primary repository's README, docs and socket API definition. Record the licence and the version studied.
- Write `docs/studies/` note with one row per idea: idea, source file, keep or skip, reason.
- File one ticket per kept idea, link it from this ticket under Related, and list the numbers in the study.
- No code changes under this ticket. Verification: the study exists and `harnez issues lint` passes.
