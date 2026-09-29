# 647 — Document foreground waiting for long-running Codex tasks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [Agentic Loop Practices](../docs/practices/AgenticLoop.md)

---

## 1. Problem & Motivation
Codex agents sometimes launch a long-running shell command in a detached/background agent session and then end the turn without a reliable way to collect its result. This can leave work orphaned or appear as a zombie, even though the shell tool already returned a live session ID that the host can continue waiting on.

Add explicit guidance for Codex: when a shell command returns a live session ID, keep that command tracked and wait for more output or completion with `functions.write_stdin` using that ID. Do not detach the task and then report completion without collecting its terminal result. Preserve existing guidance for other harnesses and for explicitly requested handoffs.

## 2. Technical Specification / Findings
The intended workflow is the normal Codex command lifecycle: `functions.exec_command` returns a session ID for a still-running command, and `functions.write_stdin` polls or sends input to that same session. This gives the host a concrete completion result and allows the task to be stopped if needed. Harnez-managed or other harness background facilities remain appropriate when their lifecycle is actually tracked and the caller is expected to remain responsive.

## 3. Implementation & Verification Plan
**Goal**: Update the canonical agent-loop guidance and its generated copy to explain how Codex agents wait on a running shell session, or stop and report if the Codex tool environment does not expose that mechanism.

Acceptance criteria:
- The guidance names `functions.write_stdin` and explains that it waits on the session ID returned by `functions.exec_command`.
- It requires collecting a terminal result or explicitly cleaning up the task before ending the session.
- The addition does not conflict with harness-specific background-task workflows or requested delegation.
- Regenerate/sync the managed documentation copy and verify the docs index/status.
