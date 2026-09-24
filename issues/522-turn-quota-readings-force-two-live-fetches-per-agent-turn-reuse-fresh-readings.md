# 522 — Turn quota readings force two live fetches per agent turn; reuse fresh readings

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Performance
**Related**: [[519-harnez-stats-agents-7-day-token-use-plan-quota-drain-and-turn-ratings]], [[507-capture-subscription-quota-snapshots-at-harnez-agent-start-resume]], [[112-agy-usage-poll-may-trigger-google-reauth-bot-detection]], [[087-generalize-flock-freshness-gate-to-codex-agy]], `internal/usage/turnquota.go`, `cmd/harnez/agent_run.go`

## Problem

519 M1/M2 force a fresh provider fetch before and after every `harnez agent start/resume`
turn, bypassing the shared live-fetch cache. 507 asked for the opposite: no extra API
pressure, reuse the cache and the collector (`harnez usage --watch`) when fresh. For agy,
every extra `/usage` poll adds to the reauth / bot-detection risk from 112. Back-to-back
turns fetch twice at each boundary.

## /goal

Each turn boundary takes at most one live fetch per provider, and none when a reading
younger than a threshold (e.g. 60 s) exists: the previous turn's "after", the shared
cache or the collector. Drain accuracy in `harnez stats --agents` stays as is (reused
readings record their age; stale ones stay `unreliable`).

## Done when

- A test shows back-to-back turns reuse the previous "after" as the next "before".
- A test shows a warm cache younger than the threshold is used without a fetch.
- agy readings follow 112's backoff.
