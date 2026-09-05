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

---

## Implementation Plan

### Current state (verified)

- `internal/telemetry/schema.go`: `schemaVersion = 2`; `schemaDDL` defines
  `tool_calls` (no `activity_category`) plus the two additive cache tables
  (`note_sanitization_cache`, `note_category_cache`), both of which the file's
  own comments call out as *not* needing a version bump because
  `CREATE TABLE IF NOT EXISTS` applies them to an existing file.
- `internal/telemetry/telemetry.go` `Open` runs `schemaDDL` then
  `checkAndStampSchemaVersion(sqlDB, path, tableExisted)`. Its comment documents
  the real 2026-08-31 incident where a stale-shape file got auto-stamped: the
  guard's whole purpose is to fail loudly and tell the user to delete the file
  rather than silently accept a mismatched column set. **An added column
  interacts directly with this guard** — see Design decisions.
- `ClassifyNotes(ctx, db, calls, classifier, now)` in `classify.go` already
  implements the exact Tier1→Tier2 cache→Tier3 batch pipeline the ticket
  describes, including dedupe and `note_category_cache` write-back. The backfill
  engine should call it, not reimplement it.
- `Filter` in `query.go` already has `Since`/`Until` (`created_at >= / <`), so
  `--days` needs no new filter plumbing — just `Since = now.AddDate(0,0,-days)`.
- `ToolCall` in `types.go` has no `ActivityCategory` field; `Query`'s SELECT
  column list would need it.

### Steps

1. **Schema** (`internal/telemetry/schema.go` + `telemetry.go`)
   - Add `activity_category TEXT NOT NULL DEFAULT ''` to `schemaDDL`'s
     `tool_calls`, plus
     `CREATE INDEX IF NOT EXISTS idx_tool_calls_activity_category ON tool_calls (activity_category);`.
   - Bump `schemaVersion` to `3` with a comment entry, and add a narrow additive
     migration in `Open`: when the table pre-existed and lacks the column, run
     `ALTER TABLE tool_calls ADD COLUMN activity_category TEXT NOT NULL DEFAULT '';`
     before `checkAndStampSchemaVersion`, then let the stamp proceed. Detect via
     `PRAGMA table_info(tool_calls)`, not by parsing an error string.
2. **Read path** (`types.go`, `query.go`, `insert.go`)
   - Add `ActivityCategory string` to `ToolCall`; include the column in `Query`'s
     SELECT and row scan. Leave `Insert` alone for now — new rows keep writing
     `''` and are picked up by the next backfill (see open questions).
3. **Backfill engine** — new `internal/telemetry/backfill.go`:
   - `ClassifyBackfillOptions{Since time.Time; Reclassify, DryRun bool; Classifier NoteBatchClassifier}`.
     Drop the ticket's separate `Days int` field — resolve days→`Since` at the CLI
     layer so the engine has one unambiguous time input.
   - `ClassifyBackfillReport{TotalScanned, UpdatedRows, Tier1, Tier2Hits, Tier3, int; Duration time.Duration}`.
   - `func (d *DB) BackfillClassifications(ctx, opts) (ClassifyBackfillReport, error)`:
     select candidate rows (`Query` + an `activity_category = ''` clause when
     `!Reclassify`), hand them to `ClassifyNotes`, then write back in one
     transaction with a single prepared
     `UPDATE tool_calls SET activity_category = ? WHERE id = ?`. `DryRun` skips
     the transaction entirely but still fills the report.
   - Chunk the scan (e.g. 500 rows) so a multi-year database does not load
     entirely into memory, and so a Tier-3 failure mid-run leaves earlier chunks
     committed rather than losing everything.
   - Tier counters: `ClassifyNotes` currently returns only categories, not which
     tier produced each. Either extend it with a parallel `[]tier` return (small,
     contained change) or drop the per-tier fields from the report. Recommend
     extending — the tier split is the main thing that tells a user whether their
     local model actually ran.
4. **CLI** — new `cmd/harnez/classify.go` registering `harnez usage classify`
   under the existing `usageCmd` (alongside `export`), *not* a new top-level
   `harnez telemetry` group: `usage export --classify` already lives there, and a
   new top-level noun for one command adds a help-surface entry for nothing.
   Flags: `--reclassify`, `--days=N`, `--since=<duration|date>`, `--dry-run`,
   `--db=<path>`. `--days` and `--since` are mutually exclusive (error, do not
   guess). Print the report as a short human summary; add `--json` only if asked.
5. **Tests**
   - `internal/telemetry/backfill_test.go` against a `t.TempDir()` db: date
     filtering; idempotence (a second non-reclassify run reports 0 updated rows);
     `--reclassify` overwriting an existing category; `DryRun` leaving the table
     unchanged while reporting a non-zero would-update count; a stub
     `NoteBatchClassifier` that errors mid-run leaving earlier chunks committed
     and no partial row corrupted.
   - Migration test: create a db with the v2 DDL, insert a row, reopen with the
     new code, and assert the column exists, the old row survives with `''`, and
     `PRAGMA user_version` is 3.
   - `cmd/harnez/classify_test.go`: flag validation, `--days`+`--since` conflict.
6. `go test ./...`, `make check`, `make install`, then a real
   `harnez usage classify --dry-run` against `~/.harnez/tool_catalog.sqlite`.

### Design decisions / tradeoffs

- **A real `ALTER TABLE` migration, against the repo's stated "no migration
  framework, just delete the file" bias.** That policy is defensible for a
  *cache*, but this ticket's entire premise is preserving historical rows —
  telling users to delete the database to get a new column would destroy the data
  the backfill exists to enrich. The compromise: one hand-written additive
  `ADD COLUMN` guarded by `PRAGMA table_info`, explicitly *not* a framework, with
  a comment saying so. Worth confirming with the user before implementing, since
  it edits the one file that argues against exactly this.
- **Reuse `ClassifyNotes` rather than reimplementing the tier ladder** in the
  backfill engine — the ticket's §2.2 pseudo-pipeline restates logic that already
  exists and would drift.
- **Denormalized column alongside the hash cache, not instead of it.** The cache
  stays authoritative per note text; the column is a materialized per-row copy so
  SQL/analytics/SQLite export (issue 208) can group by category without a join.
  Accept that the two can disagree after a reclassify until the next backfill.
- **`harnez usage classify`, not `harnez telemetry classify`** — keeps the CLI
  surface flat and colocates it with `usage export --classify`.

### Risks / open questions

- **Should `Insert` populate `activity_category` at write time** (running Tier 1,
  which is pure and cheap, on every insert)? That would make backfill a
  one-time/occasional operation instead of a permanent chore, but it puts work on
  the hot hook path. Recommend Tier 1 at insert time in a follow-up ticket, not
  here.
- Tier 3 runs a local model over potentially thousands of distinct notes on the
  first full backfill; wall-clock could be minutes. Needs progress output
  (per-chunk line) and respect for `ctx` cancellation so Ctrl-C leaves a
  consistent db.
- Interaction with issue 208: if 208 lands first its SQLite writer should read
  the new column; if this lands first, 208's export gains it for free. Neither
  blocks the other, but whichever is second should check.
- `--reclassify` over a large window will re-hit the cache, not the model, unless
  the cache is also invalidated. Decide whether `--reclassify` implies "ignore
  Tier 2 cache" (probably yes, since the stated use case is "after rule/prompt
  updates") and document it — this is the ticket's least-specified behaviour.

### Scope

**Large** — schema migration, a new engine, `ToolCall`/`Query` changes, a new CLI
command, and a change to `ClassifyNotes`' return shape. Splitting the schema +
read-path change (steps 1-2) into its own commit before the engine is advisable.
