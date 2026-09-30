# 651 — Usage Store Read Seam for Compact Output

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md)

---

/goal Make `harnez usage --compact` read normalized usage through a store API with a compatibility importer, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- Additive SQLite schema and store query return current quota/token readings with source and freshness.
- Import current state snapshots, provider caches and history without data loss or duplicate current readings.
- `usage --compact` no longer reads provider caches directly; existing output stays compatible.
- Keep all existing writers and mirrors; verify fresh, stale, missing and imported cases.

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
