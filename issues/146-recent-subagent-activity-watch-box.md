# 146 — Assess recent subagent activity in compact usage watch

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[049-running-agent-processes-watch-panel]], [[082-agent-usage-collector-daemon]], [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], [[145-orchestrator-session-skill-and-command]], `internal/usage/watch.go`

---

## 1. Problem & Motivation

`harnez usage --watch --compact` shows broad agent activity but does not tell
the user whether currently active coding tools have spawned subagents, or
which subagents were active recently. That obscures delegated work that is
often the most relevant live state during an orchestrated coding session.

Add a compact TUI box, when feasible, showing subagent activity for active
coding tools within a recent window such as the last hour.

## 2. Technical Specification / Findings

- Assess available integration points for agent-management state from Codex,
  Claude Code, AGY, and other supported coding tools. Prefer supported APIs,
  local state, or process metadata over transcript scraping.
- Define a normalized subagent activity record: parent tool/session, agent
  identity or task label, model when available, lifecycle state, and last
  activity timestamp.
- The display should use a bounded rolling window (default candidate: one
  hour), be resilient to absent integrations, and avoid falsely presenting
  ordinary agent processes as verified subagents.
- Decide whether a compact-watch box can fit without breaking existing layout
  rules, and specify omission/summary behavior in narrow terminals.

## 3. Implementation & Verification Plan

- Produce a feasibility matrix for each supported coding tool, documenting
  data source, freshness, permissions, privacy considerations, and fallback.
- Design the subagent-activity data model and collection cadence; reuse the
  existing usage collector/watch refresh architecture where appropriate.
- Implement an optional compact TUI box with recent active/completed
  subagents, clearly distinguishing unknown or unavailable sources.
- Add fixture-driven tests for window filtering, lifecycle aggregation,
  no-data behavior, and compact-layout overflow; manually verify against a
  live multi-agent session.
