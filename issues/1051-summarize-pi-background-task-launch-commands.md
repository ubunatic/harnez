# 1051 — Summarize Pi background task launch commands

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1011, 1021, 1046, 1050

## Problem and goal

When an agent launches a background task, the user-facing tool display shows the full command, including potentially long prompts. The owner wants a brief display or ellipsis there too, consistent with Pi's other truncated tool displays. This concerns the launch display, distinct from completion output (1046) and output capture (1021).

/goal Make background-task launch commands concise in the user-facing Pi display, with a clear truncation indicator and access to full details where supported, while preserving the exact command execution and useful task identity; verify in real Pi and automated tests, or stop and report if an API, user decision or permission is needed. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Re-check live code, Pi rendering APIs and recent history first. Investigate Pi's existing tool-call ellipsis and expansion behavior and what the agent and persisted transcript actually receive; don't imply visual truncation hides sensitive data. Test short, long, multiline and sensitive commands, expansion or retrieval where available, and compatibility with task labels and lifecycle.

## Decision and sprint scope

Implement launch presentation as an independent slice, without changing output capture or completion delivery (1021/1046). Use Pi's supported tool rendering and expansion where practical; keep the executed command byte-for-byte unchanged, and ensure the immediate launch result does not dump it again. Verify both collapsed and expanded views in real Pi, plus actual agent-visible tool result and transcript behavior. Any new configurable bounds belong in spec and schema. Expected write set: `.pi/extensions/background-tasks.ts`, relevant `test/pi/` tests and stubs, spec/schema and spec tests if needed, and `docs/PiAgent.md`; update if live discovery proves more is needed. Preserve existing lifecycle and coordination assertions.
