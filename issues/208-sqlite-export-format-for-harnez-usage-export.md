# 208 — SQLite Export Format for `harnez usage export`

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[204]]

---

## 1. Problem & Motivation

Issue 204 ("Sanitized Telemetry & Token Export Subcommand for Visual Analytics") shipped
`harnez usage export` with JSON output only (§2's "Output Formats" also called for a
SQLite option, deliberately deferred to keep 204 lean-sprint sized rather than building
both formats speculatively before either was proven out).

External static-site datavis (e.g. `ubunatic.com`, per 204's motivation) may prefer a
single clean SQLite file it can query client-side via WebAssembly SQLite (`sql.js`)
rather than loading and indexing a full JSON blob in the browser.

## 2. Technical Specification / Findings

- 204's export builders (`internal/telemetry.BuildExportLevel`/`ExportAllLevel`,
  `internal/usage.BuildUsageExportLevel`/`ExportHistoryLevel`) already produce the
  privacy-scrubbed, level-aware record shapes (`ExportToolCall`, `ExportPoint`). A SQLite
  writer should consume those same records rather than re-deriving scrubbing logic —
  the privacy contract belongs to 204's builders, not to the output format layer.
- Needs a `--format=json|sqlite` flag on `harnez usage export` (204's ticket text
  proposed this shape); JSON stays the default to preserve existing behavior/tests.
- Output should be a clean, single-purpose file: no leftover application tables, no
  `note_sanitization_cache` (that's harnez's own internal cache, not export content).

## 3. Implementation & Verification Plan

- Add a SQLite writer (e.g. `internal/telemetry/export_sqlite.go` /
  `internal/usage/export_sqlite.go`, or a shared `internal/export` writer taking both
  record slices) that creates a fresh on-disk sqlite file with one table per record type,
  populated from the already-scrubbed `ExportToolCall`/`ExportPoint` slices.
- Test: run an export at each privacy level, open the resulting sqlite file, and assert
  by SQL query (not just "file exists") that no raw PII string appears in any row —
  mirroring 204's "assert on serialized bytes, not just that scrubbing ran" test standard.
- Test: confirm the JSON and SQLite outputs for the same input data agree record-for-record
  (no format-specific drift in what gets included/excluded).

---

## Implementation Plan

### Current state (verified)

- `cmd/harnez/usageexport.go` `runUsageExportFull` builds
  `usageExportEnvelope{GeneratedAt, Telemetry telemetry.Export, Usage usage.UsageExport}`
  and `json.MarshalIndent`s it to `--out`. The write is the *only* format-specific
  step; everything above it is already privacy-scrubbed.
- Record shapes: `telemetry.ExportToolCall` (16 fields incl. `*int` score/exit code,
  `*int64` distilled bytes, `ActivityCategory`) and `usage.ExportPoint` (21 fields,
  two of them maps: `ModelTokens map[string]int64`, `Details map[string]string`;
  one slice: `Sources []string`).
- `modernc.org/sqlite` is already a direct dependency (`go.mod`), used by
  `internal/telemetry`. No new dependency needed.

### Steps

1. **New package `internal/export`** (`internal/export/sqlite.go`). One shared writer
   keeps the table/column contract in a single place rather than splitting it across
   the two producing packages, and avoids an import cycle between them.
   Signature: `func WriteSQLite(path string, generatedAt time.Time, calls []telemetry.ExportToolCall, points []usage.ExportPoint) error`.
   - Refuse/`os.Remove` a pre-existing `path` first so the file is always fresh
     (an append into an old export would silently mix privacy levels — decide and
     document: remove-then-create, matching the JSON writer's overwrite semantics).
   - `CREATE TABLE tool_calls (...)` and `CREATE TABLE usage_points (...)` mirroring
     the exported struct fields, plus a one-row `export_meta (generated_at TEXT,
     privacy_level TEXT, schema_version INTEGER)` table so a browser consumer can
     tell what it loaded. Nothing else — explicitly not
     `note_sanitization_cache`/`note_category_cache`.
   - Nullable columns for the pointer fields (`score`, `exit_code`,
     `distilled_bytes`) so SQL `IS NULL` matches JSON's `omitempty` absence.
   - Map/slice fields (`model_tokens`, `details`, `sources`) — decision below.
   - Single transaction, prepared statement per table.
2. **CLI flag** in `cmd/harnez/usageexport.go`: `--format=json|sqlite`, default `json`.
   Parse/validate it in `RunE` (unknown value → usage error listing both), pass it
   into `runUsageExportFull`. Keep `runUsageExport`/`runUsageExportLevel` wrappers
   unchanged so existing callers and tests stay valid; add a
   `runUsageExportFormat`-style parameter only on the `Full` variant.
   Drop the "This is a JSON-only export" line from the command `Long` text and from
   the file's header scope note.
3. **Tests**
   - `internal/export/sqlite_test.go`: write a fixture of both record slices to a
     `t.TempDir()` file, reopen with `modernc.org/sqlite`, and assert per-column
     values by `SELECT` — including a `NULL` score row and a populated
     `model_tokens` row.
   - Privacy test alongside `internal/telemetry/privacy_export_test.go`'s standard:
     for each `privacy.Level`, export a fixture containing a home path, an email,
     and a hostname, then `SELECT` every text column of every row and assert the raw
     strings do not appear. Table-driven over levels.
   - Cross-format agreement: build both outputs from the same records and assert
     row count + field-by-field equality against the JSON envelope (unmarshal the
     JSON back into the structs and compare), so neither format silently drops a
     field when a struct gains one.
   - `cmd/harnez/usageexport_test.go`: `--format=sqlite` produces a readable file;
     `--format=bogus` errors; default stays JSON.

### Design decisions / tradeoffs

- **Shared `internal/export` package over per-package writers.** The ticket offers
  both. A shared writer is the right call here: the schema is a cross-cutting
  output contract, and a per-package split would duplicate the sqlite open/tx
  boilerplate and let the two halves drift on conventions (timestamp format, NULL
  handling).
- **Map/slice columns: store as JSON text** (`model_tokens TEXT`, `details TEXT`,
  `sources TEXT`), not as child tables. `sql.js` consumers can `json_extract` these,
  and child tables would triple the schema surface for fields that are diagnostic
  rather than analytic. Document this in the package comment. (If per-model
  analytics later matter, a `usage_point_model_tokens` child table is an additive
  follow-up.)
- **Timestamps as RFC3339 TEXT**, matching `internal/telemetry/schema.go`'s existing
  `created_at TEXT` convention, so the two SQLite shapes read alike.
- **No schema migration path.** Consistent with `internal/telemetry/schema.go`'s
  stated "no migration framework" bias — an export file is regenerated, never
  migrated. `export_meta.schema_version` exists only so a consumer can reject a
  file it does not understand.

### Risks / open questions

- Does the privacy level belong in `export_meta`? It is metadata about the file,
  not user data, and a datavis site benefits from knowing it — but it also
  advertises that a `raw` file exists. Recommend including it; flag for user call.
- `modernc.org/sqlite` writes a real SQLite file, but confirm the produced file
  opens under `sql.js` (a manual check against the actual `ubunatic.com` consumer,
  not something a Go test can assert).
- `--out` currently names a `.json` file by convention. Decide whether
  `--format=sqlite` warns on a `.json` suffix or stays silent (recommend silent —
  the flag is explicit).

### Scope

**Medium** — one new package (~200 lines), one flag, three test files. No changes
to any scrubbing logic.
