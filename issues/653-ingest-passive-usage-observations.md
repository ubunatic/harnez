# 653 — Ingest Passive Usage Observations

**Status**: Closed — Claude and AGY statusline quota observations now enter the shared store with fractional precision, dedupe, bounded writes, and fresh source precedence.
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md)

---

/goal Ingest hook, statusline and AGY-meter observations through the shared usage sink with explicit provenance and precedence, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- Passive input accepts fractional quota usage, reset and observation timestamps without rounding.
- Source priority preserves concurrent observations and selects the best fresh authoritative reading.
- Statusline/hook ingestion is fail-open, secret-free and cannot block an agent turn.
- Remove direct display-only statusline/meter read paths after parity tests.

## Implementation outcome

- `internal/usage.ObserveStatusline` is the passive statusline sink; it parses only quota
  fractions/reset timestamps and calls `internal/usagestore.Store.WritePassive`.
- Claude accepts `rate_limits.five_hour.used_percentage`, `rate_limits.seven_day.used_percentage`,
  and `rate_limits.spend_limit.used_percentage` (plus `reset_at`/`resets_at`). AGY accepts
  `quota.<bucket>.remaining_fraction` (plus reset timestamps). Statusline JSON and other fields
  are never persisted.
- Spec policy: 30s identical-reading dedupe and 25ms SQLite busy timeout. SQLite observation
  selection ranks fresh values first, then provider API/structured events, authenticated CLI,
  statusline, and AGY meter/proxy estimates; exact fractions remain REAL values.
- Both `harnez statusline` and `harnez agy-statusline` render the original input unchanged and
  ignore sink/database failures. No separate display-only meter reader was removed: the AGY meter
  reader remains the registered passive collector adapter and is published to the shared store
  with collector results. Statusline formatting still reads the input payload for display.
- Verification: targeted usage/usagestore/statusline/agy tests and CLI Usage/Statusline tests pass;
  `make install` succeeds. A sample render measured 0.02s before and 0.01s after at 10ms timing
  resolution, with no measurable added latency.
