# 156 — Document how `collaboration.spawn_agent` appears in Codex Agents

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [[144-codex-subagent-model-selection-policy]], [[145-orchestrator-session-skill-and-command]], [[149-agent-specific-profiles-codex-async-wait-instruction]], `docs/AgenticLoop.md`, Codex collaboration tools

---

## 1. Problem & Motivation

Codex users can see delegated work in the Codex Agents view, but the current
project documentation does not explicitly connect that UI effect to the
`collaboration.spawn_agent` tool. The relationship was directly observed when
ticket 155 was handed to the `implement_155` agent: dispatching the tool made
that agent appear in the view.

Document the observable behavior so hosts can deliberately use delegation when
a visible, independently tracked work stream is useful, and can accurately
explain the result to users.

## 2. Technical Specification / Findings

- `collaboration.spawn_agent` starts a named child agent; it is the action
  that creates the corresponding entry in Codex Agents for the current agent
  tree.
- The visible entry provides a user-facing status surface for delegated work;
  it does not replace host responsibility for status updates, review,
  integration, or lifecycle hygiene.
- Document the relationship in the Codex-specific operational guidance (or a
  clearly linked shared orchestration document), including agent naming and
  the distinction between spawning, messaging/follow-up, and termination.
- Keep the guidance product-accurate and avoid promising UI details that have
  not been verified by the current Codex environment.

## 3. Implementation & Verification Plan

- [ ] Add concise Codex documentation identifying `collaboration.spawn_agent`
  as the mechanism that creates an entry in Codex Agents.
- [ ] Explain the practical effect: users can observe the named child and its
  progress there while the host remains responsive.
- [ ] Cross-link existing subagent lifecycle/model-selection guidance without
  duplicating it.
- [ ] Review the resulting documentation for accurate tool names and bounded
  claims about the UI.
