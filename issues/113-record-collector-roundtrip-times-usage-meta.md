# 113 — Record per-collector roundtrip times; expose via `harnez usage --meta`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[111-per-agent-collector-pipelines-independent-cadence-and-timeout]], [[112-agy-usage-poll-may-trigger-google-reauth-bot-detection]], [[105-surface-per-collector-fetch-status-in-usage-ui]], `internal/usage/agy.go`, `internal/usage/claude.go`, `internal/usage/codex.go`

## Problem

`harnez usage` shells out to external tools (`agy -p "/usage"`, and
whatever Claude/Codex collectors invoke) that can be slow, hang, time
out, or unexpectedly prompt for reauth (see issue 112). Today none of
this is measured or recorded: there's no visibility into how long each
collector's live fetch actually took on a given run or over time. This
makes it hard to answer, after the fact, questions like "is AGY's fetch
consistently slow?", "did this poll take unusually long right before a
reauth dialog appeared?", or "is this collection mechanism worth its
cost compared to the fallback (cache/history)?".

## Desired Behavior

Each collector call that shells out to or otherwise waits on an external
tool records its roundtrip time (start-to-finish wall-clock duration of
the underlying call, e.g. `exec.CommandContext` invocation) alongside
its existing result. This should be captured as part of the per-agent
usage metadata already flowing through `AgentUsage`/snapshot structs,
not a separate side channel, so it persists through the same
cache/snapshot/history mechanisms already in place.

Expose it for inspection via:

- `harnez usage --meta` — a new flag/mode that shows per-agent fetch
  metadata (roundtrip time of the most recent fetch, and ideally
  recent history/min/max/avg) alongside or instead of the normal usage
  display.
- Alternatively (or additionally) a `harnez usage meta` subcommand if
  that fits the existing command structure better than a flag — pick
  whichever matches how `--summary`/`--compact` are already structured
  in `cmd/harnez/main.go`.

## Acceptance Criteria

1. Every collector that performs a live external call (`CollectAGY`,
   `CollectClaude`, `CollectCodex`, and any future collector) records the
   duration of that call as part of its returned `AgentUsage` (or a
   sibling metadata struct persisted alongside it).
2. This roundtrip data survives through the existing collector-daemon
   snapshot/cache mechanism (`internal/usage/statecache.go`,
   `PersistAgentSnapshot`) so it can be inspected after the fact, not
   just in the process that ran the fetch.
3. `harnez usage --meta` (or equivalent) displays, per agent: last fetch
   duration, whether it hit the timeout, and whether it fell back to
   cache/history — reusing issue 105's per-collector fetch-status work
   where it overlaps rather than duplicating it.
4. Recording roundtrip time must not add meaningful overhead itself
   (a simple `time.Since` around the existing call is sufficient — no
   new external calls needed to measure this).
5. Existing tests for each collector's happy-path/fallback behavior
   continue to pass; new tests assert the duration field is populated
   and non-negative on both success and failure/timeout paths.

## Notes

This is intentionally scoped to *measurement*, not to changing any
collector's cadence or fallback behavior (see issue 111 for the
scheduling/pipeline work, and issue 112 for the reauth investigation
this data would help substantiate). The two tickets would benefit from
this data once it exists — e.g. correlating a spike in AGY roundtrip
time with a subsequent reauth dialog — but this ticket stands on its
own as a general observability improvement.

---

## Implementation Plan

### Substantial prior art already exists — extend it, don't rebuild

`internal/usage/fetchdurations.go` (issue 168) already implements ~90% of the storage half
of this ticket:

- `fetchDurationSample{EstimateMS, Samples}` persisted as JSON under the shared harnez
  state root (`~/.local/state/<app>/fetch-durations.json`), XDG-aware.
- `recordFetchDuration(homeDir, kind string, elapsed time.Duration)` — EWMA fold
  (`alpha = 0.3`), best-effort, flock-protected via `livefetchcache.go`'s generic
  `liveFetchCache[T]` machinery.
- `loadFetchDurationEstimate(homeDir, kind) (time.Duration, bool)`.
- Keys are per-"kind": `fetchDurationKindLocal` = `"local"`, or `"remote:<host>"`.
- Currently recorded from exactly one site: `watch.go:2439`, around the whole
  `CollectAll`/`CollectRemote` call, used to drive the startup splash progress bar.

So this ticket is: **widen the key namespace to per-agent, record at each collector, and
add a display surface.** Do not introduce a second timing store.

Also note: the ticket's Desired Behavior references `--summary`, which was **removed** in
issue 171 — the compact dashboard is now the default for a bare `harnez usage`. Read the
flag layout in `cmd/harnez/main.go:144-270` before designing the surface.

### Steps

1. **`internal/usage/fetchdurations.go` — per-agent kinds.**
   ```go
   func fetchDurationKindForAgent(agentID string) string { return "agent:" + agentID }
   ```
   Keys become `"local"`, `"remote:<host>"`, `"agent:claude"`, `"agent:agy"`,
   `"agent:codex"` in the same map, same file. No schema migration needed — the persisted
   payload is already `map[string]fetchDurationSample`, so new keys just appear.
   Extend `fetchDurationSample` with the fields AC #3 needs beyond the EWMA:
   ```go
   type fetchDurationSample struct {
       EstimateMS int64 `json:"estimate_ms"`
       Samples    int   `json:"samples"`
       LastMS     int64 `json:"last_ms,omitempty"`      // most recent observation
       MinMS      int64 `json:"min_ms,omitempty"`
       MaxMS      int64 `json:"max_ms,omitempty"`
       LastAt     int64 `json:"last_at,omitempty"`      // unix seconds
       LastTimeout bool `json:"last_timeout,omitempty"` // AC #3: "whether it hit the timeout"
   }
   ```
   Add `recordFetchOutcome(homeDir, kind string, elapsed time.Duration, timedOut bool)` as
   the richer entry point; keep `recordFetchDuration` as a wrapper calling it with
   `timedOut=false` so `watch.go:2439` is untouched. All fields `omitempty`, so existing
   on-disk caches load fine with zeros.

2. **`AgentUsage` — carry the last fetch duration (AC #1, AC #2).**
   `internal/usage/types.go`:
   ```go
   FetchDuration time.Duration `json:"fetch_duration_ns,omitempty"`
   FetchTimedOut bool          `json:"fetch_timed_out,omitempty"`
   ```
   These ride the existing `WriteAgentSnapshot`/`ReadAgentSnapshot` path with no changes
   (AC #2 satisfied by construction — `AgentSnapshot` embeds `AgentUsage`). Verify that.
   **Coordinate with issue 105**: that ticket adds `FetchMode`/`LastSuccessfulFetch` to the
   same struct. If 105 lands first, `FetchMode` already answers AC #3's "did it fall back
   to cache/history" and this ticket must **not** add a second parallel flag for it —
   the fields here are strictly the *timing* half.

3. **Record at each collector (AC #1) — `agy.go`, `claude.go`, `codex.go`.**
   Wrap only the external call, not the parsing:
   ```go
   start := time.Now()
   out, err := cmd.Output()          // agy.go:59-ish; the exec.CommandContext run
   elapsed := time.Since(start)
   timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
   u.FetchDuration, u.FetchTimedOut = elapsed, timedOut
   recordFetchOutcome(homeDir, fetchDurationKindForAgent("agy"), elapsed, timedOut)
   ```
   Two frictions to resolve when implementing:
   - **`homeDir` availability.** `CollectAGY(ctx, agyDir, client)` receives the *agent's*
     dir, not the user home. `fetchDurationsPath` derives from `homeDir` (or the XDG env
     var). Simplest fix: have the collectors set only the `AgentUsage` fields, and do the
     `recordFetchOutcome` call once in `collectAll` (`usage.go`), which *does* have
     `homeDir`, iterating the three results. This keeps the state-path knowledge in one
     place and avoids threading `homeDir` into every collector signature.
   - **Claude's collector may not shell out** (HTTP quota API rather than a subprocess).
     Time whatever the blocking external call is — `client.Do` — and if a collector has no
     external call at all, leave `FetchDuration` zero and record nothing rather than
     recording a meaningless microsecond.

4. **Display surface (AC #3) — `harnez usage --meta`.**
   Use a **flag, not a subcommand**: `--json`/`--raw`/`--compact`/`--proc` are all flags on
   `usageCmd` (`main.go:259-268`), and `main.go:102-119` already houses the
   mutually-exclusive-render-target validation this needs (`--compact has no effect with
   --raw`). Add:
   ```go
   usageCmd.Flags().BoolVar(&usageMeta, "meta", false, "show per-agent collector fetch metadata instead of usage")
   ```
   Reject `--meta` with `--raw`/`--json`? No — make `--meta --json` emit the metadata as
   JSON (cheap, and the natural thing for scripting); reject `--meta --raw` alongside the
   existing `--compact`/`--raw` rule.
   Rendering: a plain table in `internal/usage`, e.g. `RenderMeta(summary, homeDir) string`:
   ```
   agent    last     avg     min     max   samples  outcome
   claude   0.4s     0.5s    0.3s    1.1s      142  live
   agy      8.0s     6.2s    1.9s    8.0s       37  timeout → cache
   codex    0.2s     0.2s    0.1s    0.4s      141  live
   ```
   The `outcome` column reuses issue 105's `FetchMode` verbatim if it exists; otherwise it
   shows only `timeout`/`ok` from `FetchTimedOut` — **do not** invent a second provenance
   vocabulary here (AC #3's "reusing issue 105's work rather than duplicating it").
   Read live values from `summary.Agents[i].FetchDuration` and aggregates from
   `loadFetchDurationEstimate`-style lookups; add a `loadFetchDurationSample(homeDir, kind)`
   accessor returning the whole struct.

5. **Tests.**
   - `fetchdurations_test.go`: `recordFetchOutcome` over several observations updates
     min/max/last/EWMA correctly; `last_timeout` round-trips; a legacy JSON payload with
     only `estimate_ms`/`samples` loads without error and gains the new fields on next write.
   - Per collector (AC #5): existing happy-path and fallback tests must keep passing; add
     assertions that `FetchDuration > 0` on the success path and `FetchDuration > 0 &&
     FetchTimedOut == true` on a forced-timeout path (fake a slow command via a test script
     the way `loadstream_test.go:192` already does with `exec.CommandContext(ctx, script)`).
   - Snapshot round-trip: write and read an `AgentUsage` carrying both new fields.
   - `RenderMeta` golden-ish test with a fabricated summary and a temp
     `stateCacheDirEnv` — assert the agent rows appear and that an agent with zero samples
     renders a `—` rather than `0s`.

### Design decisions / tradeoffs

- **Reuse `fetchdurations.go`, widen the key namespace.** A per-agent timing store built
  fresh would duplicate the flock/EWMA/XDG machinery that already exists and is already
  tested. The cost is that the file now mixes two granularities (whole-fetch and per-agent)
  under one map — acceptable, and the key prefixes (`agent:`/`remote:`) keep them legible.
- **Record from `collectAll`, not from inside each collector**, to avoid threading
  `homeDir` through three collector signatures for a side-effect. The per-`AgentUsage`
  field is what each collector sets; persistence is a single caller-side loop.
- **Keep EWMA, add raw min/max/last.** AC #3 wants "recent history/min/max/avg"; a ring
  buffer of samples would answer richer questions but is a real storage design. Min/max/
  last/EWMA over an unbounded sample count is one struct and answers every question the
  ticket actually poses. If per-run history is later needed, `usage-history` is the right
  home, not this cache.
- **`--meta` displays, never collects.** It reads the persisted cache and the current
  summary; it must not trigger a live fetch just to time one.

### Risks / open questions

- `FetchDuration time.Duration` serializes as an integer nanosecond count in JSON, which is
  ugly in `--json` output. Either accept it (it's a machine surface) or store milliseconds
  as `int64` on the wire. Pick one and be consistent with `fetchDurationSample`'s `*MS`
  convention — recommend milliseconds throughout.
- Ordering with issue 105 matters more than usual: both add fields to `AgentUsage` and both
  want to display collector provenance. **Land 105 first if both are queued**, so this
  ticket's `outcome` column consumes `FetchMode` rather than inventing one and then
  reconciling.
- Issue 111 wants to tighten AGY's timeout and explicitly notes it is guessing at the right
  value. This ticket produces that number — worth landing before 111's step 1.
- AC #4 (no meaningful overhead) is satisfied trivially by `time.Since`, but
  `recordFetchOutcome` does a lock + read + write per fetch. At one fetch per agent per
  15 min that's irrelevant; do not call it per HTTP retry inside a collector.

### Scope

**Small-to-medium** — extending an existing store (~40 lines), two `AgentUsage` fields,
three collector call sites plus one persistence loop, one new render function, one CLI
flag, and a test batch. No new subsystem.
