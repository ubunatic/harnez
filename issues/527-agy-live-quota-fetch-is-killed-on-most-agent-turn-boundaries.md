# 527 — agy live quota fetch is killed on most agent turn boundaries

**Status**: Closed — 6ff686e + 0664974: agy probe 17s bound, concurrent before, fresh after; live probes 2.6-3.4s, no kills
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [[522-turn-quota-readings-force-two-live-fetches-per-agent-turn-reuse-fresh-readings]], [[519-harnez-stats-agents-7-day-token-use-plan-quota-drain-and-turn-ratings]], [[112-agy-usage-poll-may-trigger-google-reauth-bot-detection]], `internal/usage/turnquota.go`

## Problem

In `~/.harnez/agents/quota-readings.jsonl`, 12 of the last 14 agy readings on
2026-09-24 carry `"error":"signal: killed"`, both before and after 522 (c07ea58).
The live fetch is killed (likely a timeout shorter than the agy `/usage` probe), so
agy turns fall back to stale cache and `harnez stats --agents` shows agy drain as
`unavailable (unreliable)`. The report is honest, but agy drain is never measured.

## /goal

A live agy reading at a turn boundary succeeds (or fails with a named cause), and agy
turns get measured drain in `harnez stats --agents`, within 112's backoff limits.

## M1 delivered (6ff686e): agy probe no longer killed (live: 4.1 s and 2.8 s probes)

flash37 review accepted it, but a live run with a scratch build found a bug.

## M2 Pre-Work / Required Refinements

- Live session b431b05e, turn 1: "after" reused the concurrent "before" fetch
  (before age 77 ms, after age 13.9 s, turn 18 s). Result: a false zero drain.
  The concurrent "before" completes after turn start, so the "fetched after turn start"
  rule accepts it. An "after" must never reuse the same fetch as its own "before":
  compare against the before reading's fetch time, not the turn start.
- Add a test with a concurrent before that completes mid-turn.
