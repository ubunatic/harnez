# 507 — Capture subscription quota snapshots at harnez agent start/resume

**Status**: Closed — covered by 519 M1: bounded before/after readings per start/resume turn (47e78d1, forced fresh in 67c80f4); shown in harnez stats --agents
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[087-generalize-flock-freshness-gate-to-codex-agy]] (shared live-fetch cache), [[111-per-agent-collector-pipelines-independent-cadence-and-timeout]], [[112-agy-usage-poll-may-trigger-google-reauth-bot-detection]], [[113-record-collector-roundtrip-times-usage-meta]], `internal/usage/livefetchcache.go`, `internal/usage/fetchdurations.go`, `cmd/harnez/agent.go`

## Problem

`harnez agent start|resume` records tokens per turn, but not what a turn
costs against the cloud subscription's quota windows (5h/weekly limits of
Codex, Claude, AGY). Without before/after quota readings, choosing a model
by cost (see `harnez agent models`, `docs/practices/ModelRoles.md`) rests
on list prices and gut feel. For example, flash37/flash38 list as cheap
but feel expensive on the subscription.

## /goal

Every `harnez agent start` and `resume` turn stores a quota snapshot of
the turn's provider at turn start and turn end, linked to the session and
turn. The delta can then be shown per turn and per session. Constraints:

- **Fast**: each snapshot adds at most 1–2s wall time to the turn. It
  takes a bounded, best-effort path: on timeout or error it records "no
  reading", not a failure, and never blocks or fails the turn.
- **No extra API pressure**: reuse the shared live-fetch cache (issue 087)
  and the statecache/collector daemon's readings when fresh, so that agent
  turns, `harnez usage` (one-shot and `--watch`) and the collector never
  poll the same account independently or hit rate limits. The AGY
  reauth/bot-detection risk (112) applies to any added AGY polling.
- **No interference**: `harnez usage` output and the collector cadence are
  unchanged by agent snapshots.

## Open questions (verify against live code before starting)

- Can a fresh-enough cached reading (age threshold?) stand in for a live
  fetch at turn start, and must the end snapshot force a fresh fetch to
  show a delta? The cache's `MinWatchInterval` may be longer than a short
  turn.
- Which providers expose a fetch that fits in 1–2s (see 113 roundtrip
  data); for slower ones, is an async end snapshot written after the turn
  returns acceptable?
- Where snapshots live: session record vs telemetry DB.

## Done when

- Start/resume records start and end quota readings (or explicit
  "no reading") per turn; `harnez agent status` shows the delta.
- Tests prove the snapshot path honours the time bound, reuses the shared
  cache, and cannot fail a turn.
- A concurrent `harnez usage --watch` plus an agent turn issue no more
  live fetches per account than the cache interval allows.
