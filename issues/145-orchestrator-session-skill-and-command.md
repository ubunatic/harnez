# 145 — Add an orchestrator-session skill and command

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[144-codex-subagent-model-selection-policy]], [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], [[039-agentic-loop-practices-and-sprint-command]], [[064-fresh-handoff-workflow-skill-and-friction-reporting]]

---

## 1. Problem & Motivation

A session that is meant to coordinate delegated work needs an explicit,
reliable operating mode. The host should remain available in the user chat,
spawn subagents when the user requests delegation, and avoid blocking on
subagent completion unless an immediate integration step requires it.

Today those expectations are distributed across instructions and workflow
documents, making the host role easy to lose during a long session.

## 2. Technical Specification / Findings

- Provide a skill and/or command that explicitly activates orchestrator mode
  for the current session.
- The activated mode must state that the host remains responsive, delegates
  concrete user-requested subtasks, reports handoffs, and manages subagent
  lifecycle hygiene.
- It must not cause speculative delegation: user intent and normal task scope
  still govern when subagents are spawned.
- Define how the mode interacts with existing sprint and fresh-sprint
  workflows, including their non-blocking handoff rule.

## 3. Implementation & Verification Plan

- Choose a command, skill, or paired design consistent with existing runtime
  mode conventions.
- Add concise activation instructions and examples for user-requested
  delegation, status reporting, integration, and teardown.
- Test installation/discovery and confirm activation changes the session
  guidance without overwriting unrelated instructions.
- Exercise a controlled handoff to verify the host stays responsive and
  reports the selected subagent model and task scope.
