# 063 — Document Fast-Capable Subagent Model Selection

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [039-agentic-loop-practices-and-sprint-command.md](039-agentic-loop-practices-and-sprint-command.md), [062-scan-docs-across-agent-projects.md](062-scan-docs-across-agent-projects.md)

---

## 1. Problem & Motivation

Recent subagent work showed a useful pattern: for bounded implementation tasks, a smaller/fast but still capable current model is often the right default. It keeps feedback loops short while preserving enough coding quality for scoped development work.

This should be documented in the agentic workflow guidance rather than turned into a large harness capability first.

## 2. Proposed Guidance

Add a concise rule to `docs/practices/AgenticLoop.md`:

- Use a fast capable model for bounded dev subagents when the task is clear, scoped, and testable.
- Reserve frontier/heavier models for ambiguous architecture, deep debugging, security-sensitive work, or final review.
- Always give subagents a narrow task, explicit write scope, required checks, and "do not commit" unless requested.

Keep the wording short because these docs feed agent prompts.

## 3. Acceptance Criteria

- [ ] Add compact subagent model-selection guidance to `docs/practices/AgenticLoop.md`.
- [ ] Mirror the change into the root bundled copy if required by the docs layout.
- [ ] Avoid naming vendor-specific models unless the local docs already use them.
- [ ] Keep the added prompt footprint minimal.
