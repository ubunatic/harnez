# 456 — Document synchronous agent lifecycle usage and ticket-oriented communication

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: Issues 125, 149

## 1. Problem & Motivation

`harnez agent start` and `harnez agent resume` are synchronous commands that
produce low-noise output followed by the agent reply. Agents need explicit
guidance for using them with their own background-job facilities so the host
session visibly shows which agents are working and users can stop subagents or
jobs manually when needed. Agent collaboration should also remain easy to
follow by communicating mainly through tickets and short prompts when reusing
agents.

## 2. Technical Specification / Findings

Update the applicable agent-facing guidance to explain:

- Run synchronous `harnez agent start` and `harnez agent resume` commands via
  the invoking agent's background-job facilities when parallel work is wanted.
- Keep the host session's running jobs visible and user-stoppable; do not hide
  lifecycle work behind opaque polling or detached processes.
- Use tickets for durable context and short prompts for follow-up requests when
  reusing agents.

## 3. Implementation & Verification Plan

/goal: Agents consistently launch synchronous lifecycle commands through
visible, user-stoppable host-session background jobs when appropriate, and use
tickets plus short prompts as the primary durable communication path when
reusing agents.

- Identify the canonical agent-facing guidance and update it with the workflow.
- Check that the guidance distinguishes synchronous command behavior from job
  orchestration and preserves existing async-wait practices.
- Verify the updated docs are indexed and consistent with local conventions.
