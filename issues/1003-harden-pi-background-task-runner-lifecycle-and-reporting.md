# 1003 — Harden Pi background task runner lifecycle and reporting

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [Pi extension](../.pi/extensions/background-tasks.ts), [Pi agent](../docs/PiAgent.md), [665 — background self-test](665-add-harnez-selftest-skill-and-agent-selftest-cli-command-for-background-task-execution-verification.md)

---

## 1. Problem & Motivation
A `flash38:med` review of the project-local Pi background-task extension found lifecycle and reporting gaps despite passing the Harnez self-test. In particular, a task that ignores termination can outlive cancellation or session shutdown, while completed-task elapsed times continue increasing.

## 2. Technical Specification / Findings
Re-verify the current code and recent history before starting. Address task/process cleanup, including termination-resistant children; accurate final durations and signal reporting; UTF-8 output across stream boundaries; bounded task retention; and model-callable result inspection and cancellation. Review task-ID input handling. Preserve automatic agent follow-ups on completion, which are intentional. Verify speculative API and platform-specific findings rather than treating them as established.

## 3. Implementation & Verification Plan
**/goal** Harden the runner and verify task lifecycle, cleanup, reporting, and model controls with automated regression tests, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report explaining why and where to resume.

Test successful and failed commands, signal-resistant cancellation/shutdown, stable completed durations, split UTF-8 output, retained task limits, model controls, and completion notifications. Document platform limitations honestly; do not claim unkillable OS processes can always be terminated.
