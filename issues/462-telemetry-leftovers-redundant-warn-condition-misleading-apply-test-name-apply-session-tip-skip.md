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

Derive or drop `warn_condition` so only one source states the threshold; rename the test to
match what it covers; confirm or revert the session-tip skip, with a test or comment that
states why.

## Notes

- Re-verify against live code before starting.
