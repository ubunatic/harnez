# 1279 — Runner mode errors should name the taia background task route

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [taia #383](https://codeberg.org/harnez/taia/issues/383), #1270

---

## Problem
In runner mode, agent start, resume and delete errors say the action is host-owned and to use a launch-bound broker RPC, but do not name a usable route. The report points to `internal/subagent/session.go:324`.

## Goal
Provide actionable guidance to run the action as a taia background task (`mcp__taia__background_task`) when applicable, and verify the affected errors. Check live code and recent history before starting. Stop and report on a user decision or denied permission; on a loop, hard blocker or no progress, clear the goal and record a failure report and resume point.
