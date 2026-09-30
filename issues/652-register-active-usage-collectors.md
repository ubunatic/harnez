# 652 — Register Active Usage Collectors

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md)

---

/goal Move active provider, AGY-meter, and remote-load gathering behind the usage collector registry, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- Registered collectors declare identity, capabilities, cadence, timeout and cancellation.
- Claude, Codex, AGY, AGY meter and remote-load publish normalized observations.
- Slow collection does not delay independent collectors; diagnostics retain source timing/error evidence.
- Retire direct `CollectAll` reader orchestration only after equivalent snapshot and fallback behavior is verified.

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Describe the problem and why it matters.

## 2. Technical Specification / Findings
Record relevant technical details and findings.

## 3. Implementation & Verification Plan
Describe the implementation and how it will be verified.
