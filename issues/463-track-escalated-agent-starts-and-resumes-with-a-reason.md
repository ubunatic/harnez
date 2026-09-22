# 463 — Track escalated agent starts and resumes with a reason

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `harnez agent start`, `harnez agent resume`

---

## 1. Problem & Motivation

When an agent escalates work to a new or other session—often because another agent
failed—the event and its rationale are not currently tracked by the `harnez agent`
workflow. This makes escalation frequency, causes, and handoffs difficult to audit.

## 2. Goal

/goal Make `harnez agent start` and `harnez agent resume` support an explicit
`--escalated` flag with a required `--reason` value, and persist/report the resulting
escalation event so later operators can determine that the session was an escalation
and why it happened.

## 3. Implementation & Verification Plan

- Define the CLI behavior and validation for `--escalated` and `--reason`.
- Record the escalation marker and reason in the existing agent/session tracking path.
- Add tests for accepted flags, missing/invalid reasons, and normal non-escalated starts/resumes.
- Verify the recorded data is visible through the relevant status/history output.
