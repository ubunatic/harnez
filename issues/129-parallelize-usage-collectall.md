# 129 — Parallelize collectAll's per-agent usage collectors

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Performance
**Related**: [[033-usage-shared-quota-cache]], [[087-generalize-flock-freshness-gate-to-codex-agy]], `internal/usage/usage.go`

## Problem

`collectAll` (`internal/usage/usage.go`, ~lines 37-96) calls the three
per-agent collector closures — `collectClaude`, `collectAGY`,
`collectCodex` — **sequentially**, in both the `useCache` cache-fallback
branch and the live (`else`) branch. Each live collector can shell out to a
subprocess (`claude -p "/usage"`, `agy -p "/usage"`, `codex ...`) costing
roughly 3-4s of wall time per agent. Because the three calls are sequential,
a cold-cache `harnez usage` invocation (or a stale-cache tick from
`harnez agent-collector`) pays the sum of all three latencies — around
10-12s — when the three collectors have no dependency on each other and
could run concurrently for a total wall time closer to the slowest single
collector (~4s).

The per-agent caching/locking from issue 033 and the flock+freshness
live-fetch gate from issue 087 are already keyed independently per agent
(`claude`, `agy`, `codex` each get their own cache file and their own
in-process/flock serialization), so running the three collector closures
concurrently introduces no new cross-agent synchronization requirement —
each collector already only touches its own agent's cache path.

## Scope

In `collectAll`, replace the three sequential closure calls with concurrent
execution using goroutines + `sync.WaitGroup`, in **both** branches:

- The `useCache` branch: `cacheOrLive(...)` for `claude`, `agy`, `codex`
  each run in their own goroutine, writing to their own local
  `claudeUsage`/`agyUsage`/`codexUsage` variable. `fillFromHistoryIfNoQuotaWindows`
  calls stay sequential, after `wg.Wait()`.
- The live (`else`) branch: `collectClaude()`, `collectAGY()`,
  `collectCodex()` each run in their own goroutine, same pattern.

Each goroutine writes only to its own dedicated variable — no shared
mutable state across goroutines, so no mutex is needed beyond the
`sync.WaitGroup` itself. Everything downstream of the branch (the
`LastRefreshed` zero-value defaulting and the final `UsageSummary` struct
assembly) stays unchanged and runs after `wg.Wait()` in both branches.

This is a small, targeted concurrency change — no new abstraction (no
generic "concurrent collector runner"); three explicit goroutines matches
the existing goroutine+`WaitGroup` test style already used in
`internal/usage/agy_test.go`, `internal/usage/codex_test.go`, and
`internal/usage/quota_cache_test.go`.

Out of scope: any change to `agy.go`, `claude.go`, `codex.go`, or the
per-agent locking/caching internals themselves.

## Acceptance Criteria

- [ ] `collectAll` runs its three collector closures concurrently in both
      the `useCache` and live branches, via goroutines + `sync.WaitGroup`.
- [ ] No shared mutable state written from more than one goroutine (each
      writes to its own local variable only).
- [ ] `go test -race ./internal/usage/...` passes clean — no data race
      reports, and the existing concurrent-collapse tests (which assert
      exactly-one-live-call semantics per agent) still pass unmodified.
- [ ] A new or extended test in `internal/usage/usage_test.go` demonstrates
      the three collectors actually run concurrently (e.g. injecting
      artificially slow fake collectors and asserting total wall time is
      close to a single collector's duration, not the sum of three).
- [ ] A before/after timing note is captured (in this ticket or the
      resolving commit message).
- [ ] `go build ./...` succeeds; `make install` run afterward per project
      convention.
