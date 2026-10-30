# 1021 — Control Pi background task output at launch and avoid duplicate completion dumps

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1003, 1009, 1011, 1044, 1046; `.pi/extensions/background-tasks.ts`; `.pi/lib/background-task-runner.ts`

## Problem

During sitegen epic 008, the agent inspected completed background jobs while orchestrating work, then delayed completion messages displayed their full outputs again as separate user-visible messages. Fourteen large repeated notifications interrupted the conversation and were also presented to the agent. Shell redirection suggested as a workaround relies on each agent remembering to pipe output, which the owner does not want to rely on.

## Goal

/goal Give the caller an explicit, launch-time output policy for Pi background tasks, automatically capture and deliver output according to that policy without shell redirection, and verify useful failure diagnostics and on-demand retrieval without duplicate full-output notifications; or stop and report when blocked on user input or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Before implementation, re-check live plugin code, installed/global extension coordination, current behavior and recent history. The agent should select at task creation whether it wants full output delivered immediately on completion or captured to a temporary file for later inspection; in file mode a successful task should emit only a compact completion/status and retrieval location, while a failed task must make its output available for diagnosis. The owner expects the agent to read output on failure or when it chooses, rather than have every completion dump it automatically. Define bounded failure presentation without losing full output, exit/signal information or the ability to inspect results. Avoid repeating output already inspected; preserve reliable lifecycle notifications and existing cancellation semantics.

Owner clarification: long task output should go to a file so the host can choose what to read: only the final message, a bounded portion or the full transcript, or nothing when it already knows enough. Avoid loading the transcript into the host context merely to deliver a completion. Reconcile this requirement with the launch-time policy above, and distinguish generic command output from agent transcripts where a final message can be identified reliably.

The plugin, not the shell command or an agent-authored pipe, owns capture, file creation, output lifetime and access. Define safe temporary-file permissions, retention/cleanup, size/truncation behavior, compatibility and defaults for existing callers and manual tasks. Add tests for both policies, success/failure, result inspection before/after completion, large and sensitive output, cleanup and concurrent tasks; verify a real Pi session's user and agent notifications as well as automated tests. Do not claim shell redirection or a fake UI test alone proves the desired behavior.
