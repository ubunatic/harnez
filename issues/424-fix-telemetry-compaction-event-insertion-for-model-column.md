# 424 — Fix telemetry compaction event insertion for model column

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: Runtime error reported by the telemetry hook

---

## 1. Problem & Motivation

The telemetry hook fails while inserting a compaction event:

`telemetry: insert compaction event: SQL logic error: table compaction_events has no column named model (1)`

This prevents the compaction event from being recorded and may cause the hook or the surrounding workflow to fail. The failure indicates that application code and the SQLite schema disagree about the `model` column on `compaction_events`, likely because an existing database was not migrated or because schema creation is stale.

## 2. Technical Specification / Findings

- Reproduce the failing compaction-event insertion against a representative existing database.
- Identify whether the expected `model` field belongs in the schema, whether the insertion should use a different existing field, or whether a migration is missing.
- Preserve compatibility with databases created by earlier versions; do not require users to delete telemetry data.
- Ensure fresh database creation and upgrade/migration paths produce the same schema expected by the insertion code.

## 3. Implementation & Verification Plan

1. **M1 — Reproduce and locate the schema mismatch**: inspect the compaction event insert path, schema definition, and migration/version handling; add or update a regression test that fails with the reported mismatch.
2. **M2 — Correct schema compatibility**: implement the required schema or migration change and keep insertion behavior consistent for fresh and existing databases.
3. **M3 — Verify telemetry behavior**: run the targeted database/telemetry tests and `make test-q1`; confirm a compaction event containing a model can be inserted without SQL errors and that unrelated working-tree changes remain untouched.

Acceptance criteria:

- Existing databases can record compaction events containing `model` without the reported SQL error.
- Fresh databases have the schema required by the insertion path.
- Regression coverage protects both schema setup and compaction-event insertion.
