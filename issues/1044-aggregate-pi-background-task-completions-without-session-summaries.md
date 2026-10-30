# 1044 — Aggregate Pi background task completions without session summaries

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 1011, 1021; `.pi/extensions/background-tasks.ts`; `.pi/lib/background-task-runner.ts`

---

## Problem & Goal

When many Pi background tasks finish while an agent is busy, their completion notifications can queue and be handled one at a time, consuming an agent step for each. The agent may also respond to a completion with a lengthy summary of the entire session rather than treating it as a tool result arriving during ongoing work. The owner observed this particularly with multiple successful tasks; the precise queuing and delivery mechanics need verification.

/goal Make Pi background task completions economical to process: guide agents to treat them as ordinary mid-session tool results without automatically summarizing the whole session, and aggregate queued completions where practical, particularly successful ones. Preserve actionable failures, task identities and result retrieval; verify the behavior in a live Pi session, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

Before implementation, re-check live Pi and Harnez code, installed extension coordination, recent history, and issue 1021. Determine whether batching is supported by Pi's notification and agent-turn APIs, and document any limits rather than assuming the observed scheduling cause. Verify simultaneous and staggered completions while the agent is busy, mixed success and failure, no lost or duplicated results, and normal ongoing conversation. Issue 1021 separately addresses output capture and duplicate completion dumps.
