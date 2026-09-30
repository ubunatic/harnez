# 648 — Show Claude 5h and weekly used percent in the default usage view

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics

---

## 1. Problem & Motivation
Peer report from the loom session (2026-09-30): a long lean-sprint run must stop at 70% Claude quota,
but `harnez usage` shows only total tokens for Claude, no percentage. The data exists:
`harnez usage --agent claude --json` has `.agents[0].session.used_percent` (5-hour) and
`.agents[0].weekly.used_percent` (7-day), live from api.anthropic.com/api/oauth/usage.

## 2. Implementation & Verification Plan
- Show both used percentages (and reset times) for Claude in the default/compact text view, like other agents' quota rows.
- Optional: a scriptable one-liner, e.g. `harnez usage --agent claude --percent`, so agents need no jq.
- Test: rendered view contains both percentages when the JSON fields are set.
