# 458 — Move remaining telemetry SQL into `spec/` (127 follow-up)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Architecture
**Related**: [127](127-move-telemetry-sql-statements-to-spec.md), [457](457-telemetry-canonical-analytics-queries-as-tests-and-live-data-quality-checks.md), `spec/telemetry.yaml`, `docs/other/Spec.md`

---

## Problem

127 moved the schema DDL, all `INSERT`s and all `query.go` statements into `spec/telemetry.yaml`
(`c812720`, `d5178e1`). SQL literals remain in `telemetry.go` (migrations, introspection),
`classify.go`, `sanitize_cache.go`, `issuesnapshot.go`, `export.go` and `economics_query.go`.
A partial move leaves it unclear which file is authoritative.

## /goal

Decide per file whether its SQL moves into the same spec file, then move what qualifies, and
extend the AST literal guard in `sqlspec_test.go` to those files. Migration SQL (`ALTER TABLE`,
`PRAGMA`) may reasonably stay in Go if versioned steps read better there; if so, record that
decision in `docs/other/Spec.md` and exempt those files explicitly in the guard.

## Notes

- Re-verify against live code before starting; this inventory dates from 2026-09-21.
- Behavior must not change; existing telemetry tests pass unmodified.
