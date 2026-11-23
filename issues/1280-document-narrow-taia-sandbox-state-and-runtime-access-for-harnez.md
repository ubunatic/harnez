# 1280 — Document narrow taia sandbox state and runtime access for harnez

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [taia #383](https://codeberg.org/harnez/taia/issues/383), #1250

---

## Problem
Under taia auto, access to `~/.local/state/harnez/models/access.yaml.lock` is denied on each agent turn and `/run/user/1000/harnez/procs` cannot be created, breaking harnez find/exec. Telemetry also reports dropped records on every call; that symptom is separately tracked in #1250.

## Goal
Determine and communicate the narrow state and runtime directory grants needed for these workflows, or make harnez handle denied access with actionable, non-repetitive guidance. Verify behavior in taia auto without requesting broader access than necessary. Check live code and recent history first. Stop and report on a user decision or denied permission; on a loop, hard blocker or no progress, clear the goal and record a failure report and resume point.
