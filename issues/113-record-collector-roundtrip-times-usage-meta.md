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
