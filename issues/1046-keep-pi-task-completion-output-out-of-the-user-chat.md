# 1046 — Keep Pi task completion output out of the user chat

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 1011, 1021, 1044; `.pi/extensions/background-tasks.ts`

---

## Problem & Goal

Pi background-task completion currently presents the full output or transcript in the user chat. The owner wants a concise preview for both the user and host agent, with full output available on demand rather than automatically loaded into context. This is separate from issue 1021's capture policy and issue 1044's batching and agent-response guidance.

/goal Show a concise task-completion preview to the user and host agent without automatically displaying or loading the full output; preserve on-demand access, task identity, exit status and failure diagnostics. Verify both agent-visible and user-visible behavior in a real Pi session, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

Owner observed Pi's write tool render `... (7 more lines, 17 total, ctrl+o to expand)` and requests a consistent truncation/ellipsis presentation for background tasks. Investigate how Pi's write tool implements this and reuse its presentation conventions where feasible, including expansion if supported. For command output prefer the last lines over the first, with a clear indication of omitted content; verify what the agent actually receives separately from what the UI renders.

Before implementation, re-check current code, recent history and related issues. Investigate whether Pi supports separate agent and UI representations for asynchronous completion messages; do not assume visual truncation hides output from persisted transcripts or other views. Test successful and failed tasks, long and sensitive output, output retrieval and existing task workflows; document actual visibility and any API limits.
