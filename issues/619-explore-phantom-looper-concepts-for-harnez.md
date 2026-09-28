# 619 — Explore Phantom Looper concepts for Harnez

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [Phantom Looper](https://github.com/stephengpope/phantom-looper), [[039-agentic-loop-practices-and-sprint-command]], [[145-orchestrator-session-skill-and-command]], [[439-harnez-agent-chat-for-interactive-session-lifecycle-and-cross-session-subagent-reuse]]

---

## 1. Problem & Motivation

Phantom Looper combines a kanban-driven coding loop with plan/build/review agents, voice control, scheduled runs, remote persistent sessions, isolated Docker workspaces, and Git integration. Harnez already provides agent lifecycle management, ticket workflows, and a five-phase development loop, but its interface and execution model differ. A focused comparison can identify useful concepts without assuming the products should converge.

## 2. Technical Specification / Findings

- **Overlap:** both coordinate coding agents through staged work, verification, and durable task/session records. Harnez expresses this through CLI commands, issue files, and sprint guidance; Phantom Looper centers the workflow on board cards and supervised plan/build rounds.
- **Distinct Phantom Looper concepts:** voice operation, unattended cron runs, server-hosted resumable sessions, per-session container clones, and automated branch landing.
- **Harnez strengths/context:** repository-native tickets and docs, cross-agent CLI lifecycle controls, adaptable local workflows, and explicit review/process hygiene.
- The README does not establish which product concepts are practical or desirable for Harnez. Evaluate fit, dependencies, and tradeoffs before proposing implementation.

## 3. Implementation & Verification Plan

**Goal**: Compare Phantom Looper's workflow concepts with current Harnez capabilities, document the few concepts worth pursuing (or conclude none), and stop with findings if feasibility depends on a user decision or unavailable infrastructure.

- Check live Harnez code and recent history against the concepts above; identify existing equivalents and concrete gaps.
- Recommend whether any concept merits a separate ticket, with rationale and dependencies. Do not implement features as part of this exploration.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Describe the problem and why it matters.

## 2. Technical Specification / Findings
Record relevant technical details and findings.

## 3. Implementation & Verification Plan
Describe the implementation and how it will be verified.
