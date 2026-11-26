# 1302 — harnez usage: Claude range and session cost report with subsidy factor, time of day and graph

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #1300 (parked; questions and assessment), #1301 (cost inputs; **blocks** this), #204 (usage export), #969 and docs/AutoCompaction.md (Haiku compaction)

---

## Goal

/goal Add a read-only Claude report that answers the #1300 questions for a date range or the current weekly window, and verify it on real data. Done means the report shows, per session and in total: tokens (in, out, cache-read, cache-write), API cost, the share of the weekly and 5-hour limits, plan cost and the subsidy factor (API cost ÷ plan cost). It also shows the factor by local hour and over time, and its numbers pass the checks below. Stop and report when #1301 is not done, or when a design choice is the owner's. On a loop or no progress, clear the goal and write a failure report.

## Plan

- **M1 (range and session report):** a new read-only `harnez usage` view; the developer proposes the name and flags (for example `--since 7d`, `--window weekly`, `--json`).
  - The weekly window starts at the weekly `reset_at` minus 7 days.
  - Tokens come from the transcripts (`~/.claude/projects/*/*.jsonl`, the `usage` of each message, keyed by timestamp), so host sessions are counted too.
  - Quota share comes from the `quota-history.jsonl` deltas inside each session's time span. When sessions overlap, split the share by API cost and say so in the output.
  - Show a "not explained locally" line when the window's % rose more than local sessions account for. That points to other machines or web use.
- **M2 (time of day and graph):**
  - The factor per local hour of day, testing the EU-morning hypothesis.
  - The factor over time as a terminal sparkline, plus a JSON series (through `--json` or `usage export`).

## Long Haiku session (a measurement, not an extra worker)

- Run this sprint's developer as **haiku:med**. Its own session is the test.
- Before dispatch, raise the harnez threshold for that session above 100k with the user override `agent.compact_thresholds` in `~/.harnez/config.yaml`; the spec default is 80k.
- Check Haiku's context window and Claude's native autocompact first (docs/AutoCompaction.md), so the session can really pass 100k input.
- Afterwards, record:
  - turns, peak input and the turn where it crossed 100k;
  - whether the >100k rate from #1301 is applied;
  - the API cost and quota % used, against the same work at 80k compaction.
- Remove the override afterwards. No separate test workers.

## Verification

- Hand-check one session against its JSONL.
- The per-session rows add up to the range totals.
- The window's % matches quota-history and the live API.
- No silent zeros: a session without quota data shows "n/a".
- The host runs the installed binary on the owner's "this machine only" days. Tests use fixture transcripts and quota-history, never the real HOME.
