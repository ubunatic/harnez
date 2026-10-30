# 1046 — Keep Pi task completion output out of the user chat

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 1011, 1021, 1044; `.pi/extensions/background-tasks.ts`

---

## Problem & Goal

Pi background-task completion currently presents the full output or transcript in the user chat. The owner wants a brief completion indicator or ellipsis there instead, while the host agent can read the output and continue working with it. This is separate from issue 1021's launch-time capture policy and issue 1044's batching and agent-response guidance.

/goal Make task-completion output available to the host agent without displaying the full output in the user-facing chat; show only a brief indicator or ellipsis there. Preserve task identity, exit status, failure diagnostics and on-demand retrieval. Verify both agent-visible and user-visible behavior in a real Pi session, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

Before implementation, re-check current code, recent history and related issues. Investigate whether Pi supports separate agent and UI representations for asynchronous completion messages; do not assume visual truncation hides output from persisted transcripts or other views. Test successful and failed tasks, long and sensitive output, output retrieval and existing task workflows; document actual visibility and any API limits.
