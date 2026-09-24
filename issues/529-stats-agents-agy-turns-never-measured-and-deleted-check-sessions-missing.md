# 529 — stats --agents: agy turns never "measured" and deleted check sessions missing

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [[519-harnez-stats-agents-7-day-token-use-plan-quota-drain-and-turn-ratings]], [[527-agy-live-quota-fetch-is-killed-on-most-agent-turn-boundaries]], `cmd/harnez/stats_agents.go`

## Problem

After 527 (0664974), agy turn readings succeed: session chk527 has fresh, error-free
before/after readings for 2 turns in `~/.harnez/agents/quota-readings.jsonl`. Yet
`harnez stats --agents --days 1 --all` on 2026-09-24 shows:
- `agy:gemini-3.7-flash:low`: MEASURED TURNS 0, 5H DRAIN 0.0 pts.
- No rows for the deleted sessions rev522, rev527 (191b82ff, b431b05e) and chk527.
  519 M3 had fixed deleted sessions vanishing.

A 0-pt drain can be right for short turns (whole-percent quota). But zero measured
turns with good pairs, and missing rows, look like a join or filter bug.

## /goal

Every agy turn with a valid before/after pair counts as measured (drain may be 0), and
deleted sessions with readings appear under `--all`. A test uses real-shaped
readings from the agy collector.
