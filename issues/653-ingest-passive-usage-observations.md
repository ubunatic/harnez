# 653 — Ingest Passive Usage Observations

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md)

---

/goal Ingest hook, statusline and AGY-meter observations through the shared usage sink with explicit provenance and precedence, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- Passive input accepts fractional quota usage, reset and observation timestamps without rounding.
- Source priority preserves concurrent observations and selects the best fresh authoritative reading.
- Statusline/hook ingestion is fail-open, secret-free and cannot block an agent turn.
- Remove direct display-only statusline/meter read paths after parity tests.

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
