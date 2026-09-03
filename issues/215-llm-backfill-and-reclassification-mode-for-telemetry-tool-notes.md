# 215 — LLM Backfill and Reclassification Mode for Telemetry Tool Notes

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature / Telemetry
**Related**: [212-taxonomic-classification-of-telemetry-tool-notes.md](212-taxonomic-classification-of-telemetry-tool-notes.md), [204-sanitized-telemetry-and-token-export-for-datavis.md](204-sanitized-telemetry-and-token-export-for-datavis.md), [120-harnez-stats-analytical-reporting.md](120-harnez-stats-analytical-reporting.md)

---

## 1. Problem & Motivation

Issue 212 introduced the taxonomic classification pipeline (`ActivityCategory` enums: `test`, `build`, `edit`, `inspection`, `git`, `debug`, `workflow`, `config`, `other`) and content-hash caching (`note_category_cache`). When running `harnez usage export --classify`, uncached notes are classified in chunks using local model runners (`lmcoder`/`claude`) and cached.

However:
1. **Existing Database Entries Remain Unclassified in SQLite**: The classification results are currently only stored in `note_category_cache` (and included transiently in JSON exports). Stale rows in `tool_calls` itself do not have an `activity_category` column, so local SQL queries, future analytics, and SQLite exports require querying the cache or re-evaluating notes every time.
2. **Backfill & Re-evaluation Mode Missing**: There is no dedicated command or sub-mode to scan existing/stale DB entries, classify unclassified notes with the LLM/matcher pipeline, and persist the classifications back to the database.
3. **Selective Scope Needed**: Users need the ability to classify:
   - Only unclassified or stale entries (default backfill).
   - Re-classify all entries (e.g. after rule updates or improved prompt engineering).
   - Re-classify only entries from the last *X* days (e.g. `--days=7` or `--since=7d`).

---

## 2. Technical Specification

### 2.1 Schema Enhancement
In `internal/telemetry/schema.go`:
- Add `activity_category TEXT NOT NULL DEFAULT ''` to `tool_calls` table in `schemaDDL`.
- Index `activity_category`: `CREATE INDEX IF NOT EXISTS idx_tool_calls_activity_category ON tool_calls (activity_category);`.
- Handle schema evolution: if existing DB lacks `activity_category`, execute `ALTER TABLE tool_calls ADD COLUMN activity_category TEXT NOT NULL DEFAULT '';` safely on Open or schema check.

### 2.2 Storage & Backfill Engine in `internal/telemetry`
Add backfill query and write-back functionality:
```go
type ClassifyBackfillOptions struct {
    Days        int           // Filter to rows from the last N days (0 = all time)
    Since       time.Time     // Explicit start timestamp (derived from Days or set directly)
    Reclassify  bool          // If true, re-evaluate and overwrite already classified rows
    DryRun      bool          // If true, do not commit changes to DB
    Classifier  NoteBatchClassifier
}

type ClassifyBackfillReport struct {
    TotalScanned    int
    Tier1Classified int
    Tier2Hits       int
    Tier3Classified int
    UpdatedRows     int
    Duration        time.Duration
}
```

- Query target rows:
  - If `Reclassify` is false: `SELECT id, tool_name, note, exit_code, created_at FROM tool_calls WHERE (activity_category = '' OR activity_category IS NULL)` (+ date filter).
  - If `Reclassify` is true: `SELECT id, tool_name, note, exit_code, created_at FROM tool_calls` (+ date filter).
- Pipeline:
  - Run Tier 1 deterministic matcher.
  - If Tier 1 matches, assign category.
  - If Tier 1 does not match, check Tier 2 `note_category_cache`.
  - For remaining misses (or when re-evaluating with LLM), collect distinct notes and call `classifier.ClassifyBatch` in chunks (default 50-100 items), writing results to `note_category_cache`.
- Write-back:
  - Batch update `tool_calls SET activity_category = ? WHERE id = ?` inside transactions.

### 2.3 CLI Command Surface
Introduce a CLI command under `harnez telemetry` or `harnez usage`:
```bash
# Backfill unclassified records
harnez telemetry classify

# Re-classify all entries from the last 7 days
harnez telemetry classify --reclassify --days=7

# Dry run with progress output
harnez telemetry classify --dry-run
```

---

## 3. Implementation & Verification Plan

- [ ] Add `activity_category` column to `tool_calls` in `internal/telemetry/schema.go` with safe migration for existing DB files.
- [ ] Implement `BackfillClassifications(ctx context.Context, opts ClassifyBackfillOptions) (ClassifyBackfillReport, error)` in `internal/telemetry`.
- [ ] Add CLI subcommand (`harnez telemetry classify` or `harnez usage classify` with `--reclassify`, `--days`, `--dry-run`).
- [ ] Unit tests for:
  - Filtering by date (`--days`).
  - Idempotent backfill (only unclassified rows updated when `--reclassify=false`).
  - Forced reclassification overwrite when `--reclassify=true`.
  - Batch write-back transaction safety.
- [ ] Verify `make check` and `make install`.
