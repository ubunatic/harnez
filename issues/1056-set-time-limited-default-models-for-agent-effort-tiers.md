# 1056 — Set time-limited default models for agent effort tiers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [1027](1027-agent-context-modes-plain-no-skills-role-skills-for-pi-and-small-window-models.md)

---

## 1. Problem & Motivation

Temporarily select which models all new agents use for cheap, lean and smart work, without permanently replacing the user's normal defaults. Complete plain-mode issue 1027 first; its current draft permits task-specific tools, so verify that a truly tool-free invocation is supported before using it for the hello probe.

## 2. Goal & Acceptance Criteria

/goal Add time-limited model-tier defaults and optional minimal hello probes, verify expiry and agent selection, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report explaining why and where to resume.

- Provide `harnez models set [--until=time|--hours=<float>] [<cheap>/<lean>/<smart>] [--cheap=|--lean=|--smart=] [--hello]`; require a time boundary and restore normal user-defined defaults after expiry.
- Apply active tier selections to all new agents doing the corresponding work; clarify and test precedence with explicit model selection and existing defaults, persistence, expiry, invalid input and conflicting flags.
- With `--hello`, prompt each selected model at its lowest effort in plain mode, without tools, skills or injected context; report per-model outcomes. Verify provider support rather than silently using a less-isolated invocation.
- Recheck live model resolution, configuration and recent history before implementation. Specify time format and timezone, partial tier updates and probe failure behavior rather than assuming them.
