# 564 — Capture Useful Session Status-Line Usage Data

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [560 — Consolidate usage collection into a reusable Usage System and Go library](560-consolidate-usage-collection-into-a-reusable-usage-system-and-go-library.md)

---

## 1. Problem & Motivation

Some useful usage signals are only available while an agent session is active. Claude Code's status-line payload includes rate-limit `used_percentage` values and context token counts plus context-window size. These can provide fractional or session-context information that a separate quota API collector may not provide. AGY status-line payloads can include quota `remaining_fraction`. Harnez configures Codex native footer items, but does not currently receive their values through its status-line integration.

The new Usage System should be able to benefit from session-provided data without requiring an always-on collector or losing source and freshness information.

## 2. Goal

Extend usage collection to capture and expose useful status-line usage data from active sessions through the Usage System, where the agent integration actually provides it.

Done when supported status-line signals are mapped into the shared usage data model with source and observation time, can be consumed by usage apps alongside collector data, and are covered by parsing and mapping tests. Unsupported values (including values Harnez only configures Codex to display but cannot read) must not be fabricated.

## 3. Implementation & Verification Plan

- Inspect the live status-line payloads and current integrations for Claude, AGY, and Codex; record which signals are available to Harnez and their precision.
- Define how session observations enter the Usage System and coexist with API/proxy collector snapshots, including freshness and source provenance.
- Capture useful fractional quota values and context occupancy inputs where available; retain raw token counts and window size so derived fractions remain interpretable.
- Add focused parsing/mapping tests and verify that status-line collection does not cause additional API requests.
