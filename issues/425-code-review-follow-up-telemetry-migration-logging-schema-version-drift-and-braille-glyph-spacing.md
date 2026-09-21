# 425 — Code review follow-up: telemetry migration logging, schema version drift, and Braille glyph spacing

**Status**: Closed — telemetry parts delivered in 235d7da; Braille layout moved to 459, leftovers to 462
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Bug
**Related**: #424, #423

---

## 1. Problem & Motivation

A code review of recent changes (commits `c917eb6`, `7f960d2`, `aeedb20` and uncommitted working-tree modifications) identified several quality, design, and consistency issues introduced during the telemetry schema migration and font rendering updates:

1. **Stdout Pollution & Unconditional Migration Reporting in `harnez apply`**:
   - `ensureTelemetrySchema()` in [cmd/harnez/main.go](file:///home/uwe/projects/harnez/cmd/harnez/main.go) unconditionally emits `  telemetry schema: v9 (no migrations)` to stdout before the main `Applying ...` line.
   - In `checkAndMigrateSchema()` ([internal/telemetry/telemetry.go](file:///home/uwe/projects/harnez/internal/telemetry/telemetry.go)), `migrations = append(migrations, "compaction_events.model")` is appended unconditionally whenever `current < schemaVersion`, even if `model` was already present and `migrateCompactionEvents()` performed no DDL changes.
   - Resource cleanup in `ensureTelemetrySchema()` performs manual `db.Close()` calls across error branches instead of idiomatic Go `defer db.Close()`.

2. **Schema Versioning Off-by-One / Comment Drift**:
   - In [internal/telemetry/schema.go](file:///home/uwe/projects/harnez/internal/telemetry/schema.go), doc comments document schema versions up to `8: compaction_events model column (initially omitted from the migration).`, but `const schemaVersion = 9` is declared without documentation for version 9.

3. **Braille Glyph Cell Margins and Dot Spacing**:
   - In [internal/readcard/font.go](file:///home/uwe/projects/harnez/internal/readcard/font.go) (`drawBrailleRune`), dots are mapped directly to bounding pixel edges:
     `px := x + dotX[dot]*(width-1)`
     `py := y + dotY[dot]*(height-1)/3`
   - In larger fonts (e.g. 8x16 `DefaultFont8x16`), dots are rendered at columns `0` and `7` with 0 pixel horizontal margin. This causes adjacent braille runes to visually collide and merge dot patterns across character boundaries, while leaving 6 empty columns inside the cell.

4. **Untracked Case Study Dataset and Documentation Sync**:
   - [docs/README.md](file:///home/uwe/projects/harnez/docs/README.md) links to `studies/2026-09-18-codex-multimodal-context-reading-and-visual-card-economics.md`, but the study file and its associated raw benchmark logs in `docs/data/` remain untracked in git.

---

## 2. Technical Specification & Remediation

1. **Telemetry Schema & Clean Invocations**:
   - Return migration details from `migrateCompactionEvents` only when an `ALTER TABLE` was actually executed.
   - Suppress migration logs during normal `harnez apply` unless verbose logging is active or actual migrations occurred.
   - Refactor `ensureTelemetrySchema()` to use standard `defer db.Close()`.
   - Reconcile `schemaVersion` doc comments in `internal/telemetry/schema.go`.

2. **Braille Glyph Layout**:
   - Calculate inner cell margins and dot coordinates proportionally inside the font's bounding box so that Braille characters retain standard inter-glyph spacing and consistent dot sizing across all font profiles (3x5 up to 8x16).

3. **Documentation & Benchmark Tracking**:
   - Stage and commit the case study document and evaluate whether `docs/data/` session logs should be tracked or gitignored.

---

## 3. Implementation & Verification Plan

1. **M1 — Telemetry Logging and Migration Cleanup**: Fix `db.Migrations()` accuracy, remove stdout noise from `ensureTelemetrySchema()`, and update `schema.go` documentation.
2. **M2 — Braille Layout Corrections**: Adjust dot positioning and margins in `drawBrailleRune()` and update tests in `read_test.go` to assert correct bounding margins.
3. **M3 — Verification**: Run `make test-q1` and verify clean output for `harnez apply`.

## Sprint Status (lean-sprint, 2026-09-21)

Telemetry parts (M1) are delivered: migration reporting is accurate, `apply` is quiet on no-op
(`235d7da`), `defer db.Close()` and v9 comments were already in place. Remaining: M2 Braille
layout and the docs/data tracking decision.

## Closed (2026-09-21)

Telemetry parts delivered. Braille layout and the docs/data decision moved to [459](459-braille-glyph-cell-margins-in-readcard-font-go-and-docs-data-tracking-decision-425-follow-up.md); the review leftovers to [462](462-telemetry-leftovers-redundant-warn-condition-misleading-apply-test-name-apply-session-tip-skip.md).
