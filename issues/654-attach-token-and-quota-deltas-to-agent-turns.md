# 654 — Attach Token and Quota Deltas to Agent Turns

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md), [603](603-agent-models-and-agent-start-ignore-provider-quota-exhaustion.md)

---

/goal Attach provider token readings and before/after quota deltas to Harnez agent sessions and turns through the shared store, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- Record cumulative and delta token dimensions with measured/fitted/shared/unknown quality.
- Pair same-provider/pool non-reset quota observations at turn boundaries and retain provenance.
- `stats --agents`, `agent models`, and `agent start` consume the store API.
- Import then retire `quota-readings.jsonl` writes only after parity and recovery checks.
