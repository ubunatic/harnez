# 1094 — Research AutoSwarm (HKUDS) and absorb useful ideas into harnez

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 302 (research of other agent harnesses), 774 (TUI harness research), 958 (agent review subcommand), 980 (agent command epic)

---

/goal Read the open-source AutoSwarm project (github.com/HKUDS/AutoSwarm), write a findings note in docs/studies/, and file one follow-up ticket per idea that fits harnez. Done when the note lists each idea with keep/skip and the reason, and every "keep" has a ticket number. Stop and report if the repository is unavailable or licensing forbids reuse of the ideas. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

AutoSwarm (HKUDS, about 1.9k GitHub stars according to its wiki) is an open-source plug-in that lets an existing coding agent (Codex, Claude Code, Cursor, Hermes) coordinate other local agents. Its stated use cases are mixing agents' strengths, running many read or summarise tasks in parallel, and having one agent build while another reviews. These overlap with harnez's own agent start/resume, review and session work. We want to learn what AutoSwarm does well and take over the ideas that fit harnez, without copying code or changing harnez's direction.

## 2. Technical Specification / Findings

- Initial web search only (2026-11-01). Sources: the AutoSwarm wiki and README (github.com/HKUDS/AutoSwarm), and a landscape summary. Not yet verified against the repository code.
- AutoSwarm's own v1 is archived; the project was renamed from OpenOPC.
- The name "agent harness" is the user's term. Results named "auto-harness" (neosigmaai) are unrelated.
- Uncertainties to settle in the study: its licence, how it discovers and drives other agents, how it shares context and results between agents, and whether its parallel-read pattern matches harnez's Invariant 1 (one writer per workspace).

## 3. Implementation & Verification Plan

- Read the README, wiki and main code paths of AutoSwarm. Record the licence.
- Write `docs/studies/` note with one row per idea: idea, source file, keep or skip, reason.
- File one ticket per kept idea, link it from this ticket under Related, and list the numbers in the study.
- No code is changed under this ticket. Verification: the study exists and `harnez issues lint` passes.
