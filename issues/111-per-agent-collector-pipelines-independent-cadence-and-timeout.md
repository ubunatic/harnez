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

---

## Implementation Plan

### Premise correction: collection is no longer sequential

The ticket's Problem section quotes `collectAll` running
`claudeUsage = collectClaude(); agyUsage = collectAGY(); ...` in sequence. **That is no
longer the code.** `internal/usage/usage.go` (~lines 120-166) now runs all three collectors
concurrently in both branches — `sync.WaitGroup` + three goroutines for the cached path
(`cacheOrLive` per agent) and again for the `CollectAllLive` path. Issue 169's
`FetchProgressFunc` plumbing landed on top of that same structure.

Consequence: **AC #2 is already satisfied.** A slow AGY cold-start no longer delays
Claude/Codex within a tick; it only delays the *tick as a whole* from returning (the
`wg.Wait()`), which matters for the daemon's next-tick scheduling and for `--summary`'s
inline wait, not for the other agents' data.

That reduces this ticket to three genuinely-open items: per-agent **cadence** (AC #1),
per-agent **timeout** tunability (AC #4), and **cancellation** (AC #3). Re-verify the
concurrency claim above before starting — if it holds, do **not** do the "split into
per-agent pipelines" refactor the Desired Shape describes; it would rewrite working
concurrent code to buy only the cadence knob, which is much cheaper to add in place.

### Steps

1. **Per-agent timeouts (AC #4) — `internal/usage/agy.go`, `claude.go`, `codex.go`.**
   Replace the lone `const agyUsageCmdTimeout = 15 * time.Second` (`agy.go:45`) with a
   small table in one place:
   ```go
   // internal/usage/collector.go
   type agentSchedule struct {
       liveTimeout  time.Duration // bound on the live external call
       tickDivisor  int           // collect every Nth base tick (1 = every tick)
   }
   var agentSchedules = map[string]agentSchedule{
       "claude": {liveTimeout: 10 * time.Second, tickDivisor: 1},
       "codex":  {liveTimeout: 10 * time.Second, tickDivisor: 1},
       "agy":    {liveTimeout: 8 * time.Second,  tickDivisor: 2},
   }
   ```
   `CollectAGY` reads `agentSchedules["agy"].liveTimeout` instead of the const. Tightening
   AGY from 15s to 8s is the actual AC #4 win — a cold AGY fails fast to the on-disk
   snapshot (that fallback already exists, issue 104/086) instead of blocking `--summary`.
   Verify against a real cold `agy` before committing to 8s; if cold-start genuinely
   exceeds it, the right answer is a shorter timeout *plus* the fallback, not a longer one.

2. **Per-agent cadence (AC #1) — `internal/usage/collector.go` `collectTick`.**
   Do **not** give each agent its own goroutine and ticker. Keep the single ticker and add
   a tick counter, skipping agents whose `tickDivisor` doesn't divide it:
   ```go
   func RunCollector(...) {
       for tick := 0; ; tick++ { ... collectTick(ctx, ..., tick) }
   }
   ```
   `collectTick` then needs `CollectAllLive` to accept a per-agent skip set. Add
   `CollectAllLiveFiltered(ctx, homeDir, client, agents []string) UsageSummary` (or an
   options struct) and make `CollectAllLive` call it with all three. Skipped agents are
   simply not collected that tick — their previous snapshot stays on disk untouched, which
   is already the semantics every reader expects.
   This gets AC #1 ("AGY defaults to a slower cadence without a per-agent top-level flag")
   with ~20 lines and no new concurrency surface. N goroutines + N tickers would add N
   lifecycles to shut down for no additional capability.

3. **Cancellation (AC #3) — `internal/usage/collector.go`.**
   Today `RunCollector`'s `select` on `ctx.Done()` only stops scheduling; an in-flight
   `collectTick` runs to completion. Fix:
   - `collectTick` already receives `ctx` and threads it into each collector, which thread
     it into `exec.CommandContext` — so cancellation *does* propagate to the subprocesses
     once the parent ctx is cancelled. Verify this by reading each collector's ctx usage;
     `agy.go:57` derives its timeout ctx from the passed ctx, which is correct.
   - The remaining gap is that `RunCollector` **returns** before the in-flight tick's
     goroutines finish. Restructure so the tick runs in a goroutine whose completion is
     awaited on the `ctx.Done()` path:
     ```go
     done := make(chan struct{})
     go func() { defer close(done); collectTick(...) }()
     select {
     case <-ctx.Done(): <-done; return nil   // cancel propagates; wait for teardown
     case <-done:
     }
     ```
     Ensures "no lingering goroutine/process outlives the cancel" is literally true, which
     is what AC #3's test asserts.

4. **Foreground path (AC #4) — `internal/usage/usage.go` `collectAll`.**
   The cached branch's `cacheOrLive` already runs per agent concurrently; step 1's tighter
   AGY timeout is the whole fix here. Confirm no separate foreground timeout shadows it.

5. **Tests — `internal/usage/collector_test.go`.**
   - Cadence: drive `RunCollector` with a tiny interval and a fake collect hook (inject via
     an unexported package var or a `CollectFunc` field so the test doesn't shell out);
     assert claude is collected on every tick and agy on every other tick over ~6 ticks.
   - Cancellation (AC #3): a collect hook that blocks until its ctx is cancelled; cancel
     the parent mid-tick and assert `RunCollector` returns **and** the hook observed
     `ctx.Err() != nil`, with a `goleak`-style or explicit "hook returned" assertion.
     Bound the test with a short deadline so a regression fails fast rather than hanging.
   - AC #5: existing AGY fallback-to-cache tests (104/086) must pass **unmodified** —
     don't touch their assertions; if step 1's shorter timeout breaks one, that's a signal
     the timeout is wrong, not the test.

### Design decisions / tradeoffs

- **One ticker + divisor, not N tickers.** The Desired Shape asks for per-agent goroutines
  and tickers; that was written against the sequential code and buys nothing now that
  collection is concurrent, while adding N lifecycles, N shutdown paths, and N chances to
  leak. Divisor-based skipping delivers AC #1 exactly. If a future agent genuinely needs a
  cadence that isn't an integer multiple of the base interval, revisit then.
- **Hardcoded schedule table, not config.** No user has asked to tune these; a `config.yaml`
  key can be added later without changing the call sites. Premature knobs are the thing
  this repo's conventions warn against.
- **Skipped ≠ stale-stamped.** A skipped tick must not mark an agent's data stale — its
  snapshot's `FetchedAt` is what staleness is computed from, and skipping doesn't touch it.
  Coordinate with issue 105's `FetchMode` work if that lands first, so a skipped tick
  doesn't get mis-classified as `FetchModeNone`.

### Risks / open questions

- **The 8s AGY timeout is a guess.** Measure first. Issue 113 (roundtrip timing) would
  supply exactly this number — if 113 lands first, use its data instead of guessing, and
  consider making step 1 depend on it.
- Halving AGY's poll rate means AGY data can be up to 30 min old at
  `DefaultCollectorInterval` = 900s. Acceptable given quota windows are hours long, but
  it interacts visibly with issue 105/107's staleness display — check the two together.
- Injecting a fake collector for tests may require a small seam that doesn't exist yet;
  keep it unexported and package-local rather than exporting collector plumbing.

### Scope

**Small-to-medium** — materially smaller than the ticket implies, because the concurrency
half is already done. Roughly: a schedule table, a tick counter, a filtered collect entry
point, a ~10-line restructure of `RunCollector`'s select, and two focused tests. Update the
ticket's Problem section when picking this up so the stale code quote doesn't mislead the
next reader.
