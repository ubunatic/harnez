# 087 — Generalize Claude's flock+Freshness Live-Fetch Gate to Codex and AGY

**Status**: Resolved — 2026-08-29
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

## Progress (2026-08-29)

### What changed

- `internal/usage/livefetchcache.go` (new): extracted Claude's flock + `MinWatchInterval`
  freshness-gate pattern out of `claude.go` into a generic, agent-agnostic shared helper —
  `liveFetchCache[T]` (on-disk shape: `fetched_at` + a generic `payload`),
  `liveFetchCachePath`/`readLiveFetchCache[T]`/`writeLiveFetchCache[T]`, and
  `lockLiveFetchCache`/`unlockLiveFetchCache` (the same bounded-retry advisory flock Claude already
  had, unchanged in behavior). Also added `lockLiveFetchInProcess` — a `sync.Map` of per-cache-path
  `*sync.Mutex`s that serializes concurrent *same-process* callers (e.g. the statecache daemon's own
  tick racing a `--watch` fallback read for the same agent, the scenario called out in the Problem
  section) around the whole check-cache/maybe-fetch/write sequence. flock alone only ever protects
  the disk *write*, not the read-then-decide step before it, so two goroutines in one process could
  both read a cold cache and each fire a redundant live fetch before either finished writing; the
  in-process mutex closes that gap for the same-process case (cross-process coordination is still
  flock's job, unchanged and still best-effort per the original issue 033 design).
- `internal/usage/claude.go`: retrofitted to the shared helper — `quotaCache`/`quotaCachePath`/
  `readQuotaCache`/`writeQuotaCache`/`lockQuotaCache`/`unlockQuotaCache`/
  `quotaCacheLockRetries`/`quotaCacheLockDelay`/`quotaCacheFilename` removed in favor of the generic
  versions plus a new `claudeQuotaPayload{Session, Weekly}` type. Behavior-neutral: same cache
  filename (`harnez-quota-cache.json`), same freshness/lock/stale-fallback logic, plus the new
  in-process mutex.
- `internal/usage/codex.go`: `CollectCodex`'s live-fetch section wired to the same gate —
  `codexQuotaPayload{Session, Weekly}`, warm-cache short-circuit, flock-guarded write on success,
  and (new, mirroring Claude) a stale-cache fallback labeled `(stale)` on fetch failure — Codex
  previously had no such fallback at all.
- `internal/usage/agy.go`: `CollectAGY`'s live-fetch section wired to the same gate —
  `agyQuotaPayload{ModelGroups}` (AGY's quota shape is a set of named model groups, not
  Session/Weekly windows). Added `findAGYPortsFn` (a package-level var wrapping `findAGYPorts`) so
  tests can stub port discovery, which otherwise depends on real `/proc` entries for a running `agy`
  process, and point the gate at a mock RPC server.

### Tests

- `internal/usage/quota_cache_test.go`: existing Claude tests updated to the new generic
  symbols/types (behavior-neutral); added `TestCollectClaudeConcurrentCallsDoNotDoubleFetch`.
- `internal/usage/codex_test.go`: added `TestCollectCodexUsesWarmDiskCache`,
  `TestCollectCodexExpiredCacheTriggersLiveFetch`, `TestCollectCodexConcurrentCallsDoNotDoubleFetch`.
- `internal/usage/agy_test.go` (new): `TestCollectAGYUsesWarmDiskCache`,
  `TestCollectAGYExpiredCacheTriggersLiveFetch`, `TestCollectAGYConcurrentCallsDoNotDoubleFetch`.
- All three "concurrent calls" tests fire `n=8` goroutines at a cold cache and assert exactly one
  live HTTP/RPC call reaches the mock server, confirming the in-process mutex — not just the
  freshness check — is what collapses same-process concurrent callers.
- `go build ./...`, `go vet ./...`, `make check` (`go test ./...`), `go test ./internal/usage/...
  -race` (needed since the new tests exercise real goroutine concurrency), and `make install` all
  pass.

**Status**: Resolved. `claude.go`'s gate is extracted into a shared, generic helper and applied
uniformly to `codex.go` and `agy.go`, including cross-process flock coordination (issue 033's
original mechanism, unchanged) and a same-process in-process mutex (new) that makes "concurrent
calls don't double-fetch" actually hold for same-process callers rather than only for calls spaced
further apart than a network round-trip.
