# 655 — Consolidate Legacy Usage Stores Safely

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactor
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md)

---

/goal Consolidate usage snapshots, cache and history writers into SQLite-generated compatibility mirrors without losing recoverability, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- SQLite produces compatible current snapshots/history exports for legacy consumers during transition.
- Verify current, historical, offline and rollback parity before deleting a writer.
- Archive legacy data with a manifest and import path; never silently delete user data.
- Remove provider-cache/history/turn-JSONL writers only after all readers use the store API.

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
