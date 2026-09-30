# 655 — Consolidate Legacy Usage Stores Safely

**Status**: Closed — Isolated internal/usage tests with temporary HOME/XDG roots and a telemetry path guard that rejects host storage. Converted cache fixtures to seed the authoritative SQLite store; retained JSON mirrors only as compatibility fallback for non-quota snapshot metadata. Fixed host DB locking that made verbose package tests exceed five minutes: go test -timeout 120s -v ./internal/usage passed in 10.13s. Focused telemetry/usagestore tests and make install passed. The single final make test-q1 passed, including internal/usage in 6.465s. Changes committed as d35a1999.
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
