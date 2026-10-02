# 680 — Probe Codex cross-task messaging for agent send

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 656 (`harnez agent send`)

---

/goal Probe and document Codex cross-task discovery and message delivery for #656, or stop and report when blocked by unavailable session access or a denied permission.

## 1. Problem & Motivation
Issue 656 needs a proven Codex delivery method for `harnez agent send`. In this Codex environment, native task tools can list tasks and send a follow-up by thread ID, including to an interactive task that is not a child in the current subagent tree. The send call returned the target thread ID, but that alone does not confirm the recipient processed or answered the message.

## 2. Technical Specification / Findings
The available native operations include listing tasks, reading a task by ID, sending a follow-up to a task by ID, and waiting for task updates. Task titles are labels; thread IDs address the target. Determine which supported Codex interface exposes these operations to Harnez, how named agents map to Codex thread IDs, and what the interface guarantees for active versus idle targets. Do not treat an accepted send as confirmed delivery.

## 3. Implementation & Verification Plan
Canary-test a harmless message to a separate Codex task, confirm its receipt or response, and record targeting, active/idle behavior, failure cases, and confirmation semantics in `docs/studies/` for issue 656. Recommend how `harnez agent send` should use the verified interface.
