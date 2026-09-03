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
