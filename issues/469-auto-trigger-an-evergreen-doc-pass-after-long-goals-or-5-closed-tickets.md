# 469 — Auto-trigger an evergreen doc pass after long goals or 5+ closed tickets

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Workflow / Docs
**Related**: `/evergreen` skill, `docs/AgenticLoop.md` (Phase 5), [467](467-recommend-a-read-only-plan-first-initial-prompt-in-sprint-skills-and-docs-without-a-template.md)

---

## Problem

In the 2026-09-21 session, learnings only reached the evergreen docs because the user ran
`/evergreen` late. Long goals and multi-ticket runs otherwise end with tickets and a roadmap
refresh but no broader doc update (`docs/Telemetry.md` had no store internals until then).

## /goal

After a long-running goal, or once 5+ tickets are closed in a session, and when no evergreen or
similar broader doc update was requested, the agent runs (or proposes) an `/evergreen` pass
before declaring the goal done. Decide the mechanism during planning: a rule in the sprint
skills/AgenticLoop Phase 5, or a session-state counter surfaced by the session-tip hook.

## Notes

- Threshold and "long-running" definition are open; start with the ticket count and record what
  proves useful.
- Must not trigger when the user already asked for an evergreen or doc update.
- Re-verify against live code before starting.
