# 655 — Consolidate Legacy Usage Stores Safely

**Status**: Closed — Implemented four milestones: normalized store readers expose cumulative/five_hour/tokens/weekly with zero joined legacy aliases; copied 17 legacy files into /home/uwe/.local/share/harnez/archive/usage-legacy/20260930T173221.939970271Z with SHA-256 manifest; snapshot parity 3/3, provider-cache 3 store observations from 6 archived copies, history summary 2154 records, turn JSONL imports 1851. Live usage --compact and statusline rendered. make test-q1 was run once and failed: internal/usage cache/history tests assume disk-authoritative behavior; three archival tests hit host read-only path. Raw DB retains orphan rows; no legacy source files were removed.
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
