# 111 — Per-agent collector pipelines: independent cadence, stricter timeout, and cancellation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[104-agy-quota-collector-requires-live-process-poll-coincidence]], [[087-generalize-flock-freshness-gate-to-codex-agy]], `internal/usage/collector.go`, `internal/usage/usage.go`, `internal/usage/agy.go`

## Problem

`CollectAll`/`CollectAllLive` (`internal/usage/usage.go:37`) fetch
Claude, AGY, and Codex usage **sequentially** inside one function call:

```go
collectClaude := func() AgentUsage { return CollectClaude(ctx, claudeDir, client) }
collectAGY := func() AgentUsage { return CollectAGY(ctx, agyDir, client) }
collectCodex := func() AgentUsage { return CollectCodex(ctx, codexDir, client) }
...
claudeUsage = collectClaude()
agyUsage = collectAGY()
codexUsage = collectCodex()
```

The background daemon (`RunCollector`, `internal/usage/collector.go:26`)
drives this with a single shared `time.Ticker` at one cadence
(`DefaultCollectorInterval` = 900s) and calls `CollectAllLive` once per
tick, blocking until all three collectors return before the next tick
can fire. `CollectAGY` (`internal/usage/agy.go:151`) shells out to
`agy -p "/usage"` (issue 104) with its own 15s timeout
(`agyUsageCmdTimeout`), but that timeout only bounds AGY's own call — it
doesn't let AGY's collection run independently of Claude/Codex, and a
foreground call (`harnez usage --summary` with a stale/cold cache) still
pays this cost inline and synchronously (`usage.go:51-56`).

Symptoms reported by the user:

- AGY's `agy` CLI is slow to cold-start, so any tick that needs a live
  AGY fetch is slow, and because collection is sequential, this stalls
  the Claude/Codex reads in the same tick too.
- There's no way to give AGY a longer/looser cadence (e.g. poll every
  other tick) independent of Claude/Codex's cadence, since all three
  share one ticker.
- Stopping the usage watcher/daemon doesn't proactively cancel an
  in-flight per-agent fetch beyond that agent's own internal timeout —
  there's no per-agent context wired to the watcher's lifecycle that
  tears down all in-flight agent calls together on stop.

## Desired shape

Each agent source (Claude, Codex, AGY, and any future agent) should run
as its own independent pipeline, not a step in one shared sequential
function:

1. **Own goroutine + own ticker per agent.** Each agent's collector runs
   on its own cadence, derived from (but not required to equal) the
   shared base interval — e.g. AGY ticks every 2nd interval instead of
   every interval, since its CLI is slower and the data changes less
   often, while Claude/Codex keep the current cadence.
2. **Stricter, source-appropriate timeout with fast stale-fallback.**
   When a live fetch (e.g. `agy -p "/usage"`) doesn't return inside a
   short bound and no such process is already running, fall back to the
   last on-disk snapshot (already implemented, issue 104/086) quickly
   rather than tying up the tick — this should be tunable per agent
   rather than one constant shared across the collector layer.
3. **Cancellation tied to the watcher's lifecycle.** Each per-agent
   pipeline's context should derive from the same parent context the
   watcher/daemon uses. Stopping the watcher (ctx cancel) must cancel
   all in-flight per-agent fetches, not just stop scheduling new ticks —
   today `RunCollector`'s `ctx.Done()` only stops the ticker loop; an
   in-flight `collectTick` call still runs to completion (bounded only
   by AGY's internal 15s timeout).
4. **No change to the tool-invocation contract**: this is about
   scheduling/concurrency around the existing per-agent collectors
   (`CollectClaude`, `CollectAGY`, `CollectCodex`), not about changing
   what each one shells out to or how it parses output.

## Acceptance Criteria

1. Each agent's live collection runs on an independently configurable
   cadence; AGY defaults to a slower cadence than Claude/Codex without
   requiring a separate top-level flag per agent.
2. A slow or hung per-agent fetch (e.g. AGY cold-start) does not delay
   or block the other agents' collection in the same tick window.
3. Cancelling the watcher/daemon's parent context promptly stops all
   in-flight per-agent fetches (verified via a test that cancels mid-tick
   and asserts no lingering goroutine/process outlives the cancel).
4. `harnez usage --summary`'s foreground stale-cache-fallback path
   (`usage.go:51-56`) benefits from the same per-agent timeout tightening
   — a cold AGY should fail fast to the stale snapshot rather than
   blocking the whole `--summary` call.
5. `go test ./...` / `make check` pass; existing AGY fallback-to-cache
   tests (issue 104/086) keep passing unmodified in behavior.

## Notes

This is purely a scheduling/concurrency refactor of the collector layer
— it does not change any agent's fetch mechanism (canary-verified paths
like `agy -p "/usage"` stay as-is, see issue 104). See
`docs/studies/2026-08-28-usage-collector-daemon-architecture.md` for the
existing per-agent quota-source architecture background this builds on.
