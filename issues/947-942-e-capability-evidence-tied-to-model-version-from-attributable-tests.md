# 947 — 942 E: capability evidence tied to model version from attributable tests

**Status**: Backlog — epic 942 re-cut (2026-10-25): no attributable capability-test source yet; promote when one exists
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic), 945, 941 (depends on both)

---

/goal Capability observations are stored per exact model version, harness and tier with a test definition,
expected and actual result and evidence IDs; aggregation marks an assumed capability proven, disproven or
disputed, and a new version resets applicability; tests cover attribution, conflicts and version isolation. Or
stop and report when no attributable test source exists yet. Circuit breaker: on a loop or no progress, clear the
goal and write a failure report here.

## Notes

Today telemetry `ToolCall` (`internal/telemetry/types.go:8`) and `harnez rate` (`cmd/harnez/rate.go:253`) carry
no model capability; sessions (`internal/subagent/session.go:16`) carry model and tier. Generic low ratings must
not disprove a capability; only attributable capability tests do.
