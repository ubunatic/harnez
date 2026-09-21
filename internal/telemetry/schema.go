package telemetry

// schemaVersion is bumped whenever schemaDDL's shape changes in a way
// CREATE TABLE/INDEX IF NOT EXISTS can't apply to an already-existing
// file (e.g. issue 118's distilled_bytes NOT NULL -> nullable change,
// which silently did nothing against a pre-existing db until the file
// was manually deleted). Stored via SQLite's built-in PRAGMA user_version
// (see Open in telemetry.go) rather than a metadata table, so there's
// nothing to create/migrate for this check itself. This is deliberately
// NOT a migration framework — still no migration framework, per this
// repo's "just change the code" bias — it only turns a confusing runtime
// constraint error (or worse, a silent schema mismatch) into one clear
// message telling the user to delete the file, since a single-user local
// telemetry cache has nothing worth an automated migration path for.
//
// 1: issue 116's original shape (distilled_bytes INTEGER NOT NULL DEFAULT 0).
// 2: issue 118's fix (distilled_bytes made nullable).
// 3-4: additive provider token columns used by Codex telemetry.
// 5: additive compaction_events and session_boundaries tables.
// 6: ordered per-turn and cumulative provider token snapshots.
// 7: versioned pricing inputs and immutable compaction economics results.
// 8: compaction_events model column (initially omitted from the migration).
// 9: current schema marker after the compaction_events model migration.
const schemaVersion = 9

// schemaDDL is the single source of truth for the tool_calls table shape
// (per docs/other/Spec.md's "spec files are the single source of truth"
// principle, applied here to SQL DDL rather than a YAML spec file — Go
// code in this package must not hardcode a parallel copy of the field
// list or constraints below; it reads/writes generically off this shape
// and lets SQLite itself enforce constraints like the score range at
// write time). Idempotent: CREATE TABLE/INDEX IF NOT EXISTS, run on every
// Open — no migration framework, per this repo's "just change the code"
// bias (docs/other/Spec.md / docs/lang/Go.md).
var schemaDDL = mustTelemetrySQL().Schema
