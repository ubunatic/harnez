# 087 — Generalize Claude's flock+Freshness Live-Fetch Gate to Codex and AGY

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactor
**Related**: [[033-usage-shared-quota-cache]], [[082-agent-usage-collector-daemon]], [[086-offline-degraded-cache-snapshot-masks-live-data]]

## Problem

`internal/usage/claude.go` has a cross-process protection (issue 033) around its live quota
fetch: before calling the Anthropic usage API, it checks an on-disk `quotaCache` and skips the
network call entirely if a reading younger than `MinWatchInterval` (30s) already exists; an
advisory flock coordinates concurrent writers of that cache (best-effort — if the lock can't be
acquired, the fetch still happens, it just isn't persisted).

`internal/usage/codex.go` and `internal/usage/agy.go` have no equivalent — every live collect
calls their respective endpoints (`chatgpt.com/backend-api/wham/usage`, the local AGY RPC)
unconditionally, with no short-circuit and no cross-process coordination at all.

[[082-agent-usage-collector-daemon]] added a second, separate caching layer (the shared
`statecache.go` snapshot store, deliberately lock-free — the daemon is its sole writer) that sits
*above* the per-agent collectors and reduces how often live collection happens at all, as long as
the daemon is running and its cache is fresh. But that layer doesn't close the gap: if the
daemon's cache is stale/missing and multiple readers (e.g. the daemon's own tick plus a `--watch`
fallback) call the live collector concurrently, Codex/AGY have nothing preventing simultaneous API
hits — unlike Claude, which still has its `MinWatchInterval` short-circuit underneath.

## Architectural Note (considered and rejected)

Considered migrating to an embedded single-file DB (SQLite or DuckDB) so cross-process locking is
handled by the library instead of hand-rolled flock. Rejected for now: the actual problem is
narrow (a cross-process mutex + last-fetch-timestamp gate over ~3 small records), not a
data-modeling/query problem — there's no join/aggregation need today. SQLite would work but adds
a new dependency, schema, and migration story for a problem a ~20-line shared lock helper already
solves. DuckDB is the wrong tool for this problem entirely (OLAP engine, not a lightweight
cross-process cache/mutex store) — it could be worth revisiting for [[084]]'s aggregate/historical
queries later, but that's a different problem (querying accumulated history) from this one
(gating live API calls).

## Desired Behavior

- Extract Claude's flock + `MinWatchInterval` freshness-gate pattern from `claude.go` into a
  shared helper (e.g. in `statecache.go` or a small new file in `internal/usage/`).
- Apply it uniformly so `CollectCodex` and `CollectAGY` also skip their live network/RPC call when
  a recent-enough on-disk reading already exists, coordinated via the same flock mechanism Claude
  already uses.
- No new external dependency; reuse and generalize existing, already-tested code.

## Next Steps

- Extract the shared helper, retrofit `claude.go` to use it (should be behavior-neutral for
  Claude), then wire `codex.go` and `agy.go` to the same gate.
- Add tests mirroring the existing Claude quota-cache tests, applied to Codex/AGY collectors.
