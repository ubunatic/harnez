# 462 — Telemetry leftovers: redundant `warn_condition`, misleading apply test name, apply session-tip skip

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Cleanup
**Related**: [457](457-telemetry-canonical-analytics-queries-as-tests-and-live-data-quality-checks.md), [425](425-code-review-follow-up-telemetry-migration-logging-schema-version-drift-and-braille-glyph-spacing.md), `spec/telemetry.yaml`, `cmd/harnez/main.go`

---

## Problem

Three small review findings from the 2026-09-21 telemetry sprints:

1. `quality_checks` in `spec/telemetry.yaml` carry both `warn_condition` text and the
   authoritative `warn_above_percent`; the text can drift from the number.
2. `TestEnsureTelemetrySchemaMigratesBeforeApply` no longer stamps the DB stale, so it covers
   the no-op path; the migration path is `TestApplyCmdMigratesLegacyCompactionSchema`.
3. The 425/428 worker made `apply` skip the session-tip DB reads in `cmd/harnez/main.go` so the
   tip hook cannot open the DB before the migration. Not asked for by the ticket.

## /goal

Decided 2026-09-21:

- Drop `warn_condition` from `spec/telemetry.yaml`, the loader (`sqlspec.go`), the JSON schema and
  the `sqlspec_test.go` check; `warn_above_percent` is the single source. Keep the human-readable
  wording in the check's `description` or a `#` comment.
- Rename the test to match what it covers (no-op path).
- Keep the `apply` session-tip skip in `cmd/harnez/main.go`. Add a comment saying why (the hook
  must not open the DB before `apply` migrates it) and a test that pins it.

## Notes

- Re-verify against live code before starting.
