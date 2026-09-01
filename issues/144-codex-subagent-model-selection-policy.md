# 144 — Ensure Codex uses the correct model for subagents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], [[149-agent-specific-profiles-codex-async-wait-instruction]] (establishes the per-agent profile mechanism this policy's content likely migrates into), `AGENTS.md`, Codex subagent dispatch configuration

---

## 1. Problem & Motivation

Codex subagents can inherit the host model unless a model is explicitly set.
That can assign an unsuitable model to implementation, review, or advisory
work and makes the effective model choice opaque to the user.

## 2. Technical Specification / Findings

- Define a project policy for choosing a subagent model and reasoning effort
  by task class: implementation, review, focused investigation, and simple
  mechanical work.
- The dispatcher must explicitly select the intended model when the policy
  calls for one, rather than relying on accidental inheritance.
- User-requested model overrides take precedence and must be reported when a
  subagent is dispatched.

## 3. Implementation & Verification Plan

- Document the selection policy in the canonical agent instructions or a
  referenced practice document.
- Add a dispatch helper or guardrail where supported, so implementation work
  receives the policy's coding-capable model and effort level.
- Verify spawned-agent metadata records the requested model and effort, and
  cover fallback behavior when an explicit model is unavailable.
- Keep routine tasks economical while reserving stronger coding models for
  cross-layer debugging and higher-risk implementation work.
