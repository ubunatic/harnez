# 924 — agent models takes 24s: per-row quota availability query on the usage store

**Status**: Closed — 7f4d55cb: agent models 0.81s (was 24.1s), memoized provider availability and streamed currentFor through the provider/window index
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 829 (usage store), 507/519 (quota readings at agent start)

---

/goal `harnez agent models` finishes in under 1s on this host with the same output, and the quota lookup it
shares with `harnez agent start` is no longer slow; verified by a test that counts store queries per provider and
a timed run of the installed binary; or stop and report. Circuit breaker: on a loop or no progress, clear the goal
and write a failure report here.

## Findings (host, 2026-10-25)

- `harnez agent models`: 24.1s real, 20.7s user (CPU-bound). `--names`: 0.05s; `agent list`: 0.3s.
- Stack dump at 8s: `agent_cmds.go:152` → `providerAvailability` (`agent_run.go:178`) →
  `usage.CachedProviderQuotaAvailability` (`statecache.go:72,87`) → `usagestore.(*Store).currentFor`
  (`store.go:996-1002`), inside sqlite.
- Cause: one availability lookup per model row (29 rows, 3 providers), each running a `ROW_NUMBER() OVER
  (PARTITION BY provider,pool,window_key ...)` over the provider's whole quota history (~0.8s each).

## Decision (host)

1. Look up availability once per provider (or provider+model where the result really depends on the model)
   per command run: memoize in `agent models`, not a global cache.
2. Make `currentFor` cheap on its own: the history keeps growing, and `agent start` pays the same cost. Use
   an `EXPLAIN QUERY PLAN` to pick the fix (index matching the partition/order, or bound the scan to the
   newest rows per window); same results, checked by the existing store tests.
