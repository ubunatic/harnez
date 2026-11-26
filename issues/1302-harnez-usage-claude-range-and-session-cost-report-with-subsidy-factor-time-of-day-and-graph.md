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

## Note (host, 2026-11-26): compaction limits for the long Haiku session

- The owner's `~/.claude/settings.json` sets `modelSettings` → `autoCompactWindow: 200000` for Haiku, Opus and Sonnet 5.5. The owner set Sonnet's through the Claude TUI on 2026-11-26.
- harnez agent runs `claude -p` with the same HOME, so this applies there too. docs/AutoCompaction.md measured that `-p` honours the window. With the default buffer of about 33k, native compaction should start near 167k; that figure is an estimate.
- The harnez threshold (80k for Haiku, from spec `compact_thresholds`) compacts between turns and fires first.
- For a session over 100k: raise the harnez threshold for Haiku above about 170k. The native 200k window then sets the ceiling, so the session can pass 100k and stay below about 167k.

## Note (1301 M2, 2026-11-26): coverage of per-turn rows

- Per-turn token and cost rows exist only for `harnez agent` turns (`cmd/harnez/agent_run.go`). A host Claude Code session (for example bd2185a4) writes no turn rows by design; the report must show it as "n/a", never as zero.
- Per-call Claude pricing (1301 M1) prices only calls with the per-call cache split. Older turns without the split stay unknown (nil cost).
- Open: the cumulative `turn_token_usage` row still counts cache writes in `cached_input_tokens`.

## Carried over from #1301 (host, 2026-11-26)

- **Live plan share not yet seen.** No canary turn has yet shown a live `paired` quota outcome with a filled `subscription_price_share_micros`. Every short canary reused one cached usage snapshot. Since M4 these are classified `unchanged_snapshot`, and before M4 they paired as 0 deltas with a null share. The long haiku:med session in this sprint must show at least one `paired` row with a nonzero delta and a share that matches a hand calculation: delta fraction × weekly plan share.
- **Turn records keep the alias.** The session JSON `TurnRecord.Model` (`populateTurnCostRecord`) still stores the session alias (`opus`). The DB rows store the reported id since #1301 M4. Align the JSON when the report reads turn records.
- **Concurrent sessions skew per-turn deltas.** Other Claude sessions on the same plan (for example ../splitty, ../taia) move the quota during a turn. Per-turn deltas are therefore an upper bound, and quota per dollar has to be analysed as totals over time windows.
- **Plan price fixed (owner, 2026-11-26).** The owner pays Pro yearly: €180 net + 19% VAT = €214.20. `spec/agent.yaml` now stores €15/month net, replacing the earlier €18 default; that is $16.4025/month at the ECB rate.
  - Decision: use net, because the API list prices also exclude VAT, so the subsidy factor compares like with like.
  - The report should say "net". Showing the owner's gross cost as a second line would be useful.
- **The share code has the same reset-jitter bug that #1301 M4 fixed in SQL.** `ComputeTurnQuotaShare` (`internal/usage/turnquota.go:222`) calls `sameResetMarker`, which uses exact `time.Equal` (`turnquota.go:165`). The canary resets differ by about 0.18 s, so the in-session share is nil even when the DB delta pairs.
  - Fix: use the `turn_quota_pairing.reset_tolerance` from `spec/usage.yaml`.
  - This is a prerequisite for the live plan-share check above.
