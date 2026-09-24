# 527 — agy live quota fetch is killed on most agent turn boundaries

**Status**: Open
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
