# 651 — Usage Store Read Seam for Compact Output

**Status**: Closed — Compact collection now persists normalized quota and token observations and renders through the SQLite store query.
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

## Outcome

Added the additive `usage_observations` / `quota_windows` schema and `internal/usagestore` query/write API. Compact collection continues through `CollectAll`, writes current provider readings as provenance-tagged observations, then renders the current-view store result. First use backfills state snapshots, provider quota caches, and quota history; database errors preserve the direct collected summary.

Verification: touched packages pass; targeted compact CLI tests pass; the documented unrelated `TestApplyDiffCLI_ComponentsDocsOnlyRoundTrip` config drift remains. The installed compact output was visually compared with the pre-change binary; quota rows matched, while load values varied naturally.
