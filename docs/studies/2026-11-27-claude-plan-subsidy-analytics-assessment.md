---
kind: topic
date: 2026-11-27
tickets: [1300, 1302, 1303, 1305, 1306, 1307]
---

# Claude plan-subsidy analytics: how well the six questions are answered now

The six owner questions are in ticket 1300 (parked 2026-11-26). Tickets 1301, 1302 and 1303 have since delivered most
of what they need. This assessment reruns the installed binary on live data from this machine (2026-11-27, about
12:00 CET) and checks the numbers by hand.

## Data used
- `harnez usage history report --window weekly` and `--since 7d` (text and `--json`), installed binary at `ab734966`.
- Aggregate counts from `telemetry.sqlite` `compaction_events` and `compaction_economics` (no message text).

## The six questions

| Q | Question (1300) | Answered? | Command or gap |
|---|---|---|---|
| 1a | Tokens in the last 7 days | **Yes** | `report --since 7d`: 5199 calls; in 10.7k, out 6.19M, cache read 414M, cache write 19.2M |
| 1b | Tokens in the weekly quota window | **Yes** | `report --window weekly`: window 2026-11-21 11:00 .. 2026-11-27 11:59 UTC; nearly the same totals as 7d (5197 calls) |
| 2 | Plan % per session | **Partly** | Weekly share per session (`QUOTA %`, fractional). Most sessions are below 1 pp, so their factor is `n/a`. The 5-hour window is not in the report |
| 3 | API ÷ plan cost for the range | **Yes** | Attributed: $72.95 API for 39 pp → **49.4x**. Whole window: $74.84 for the 53 pp rise → **37.3x** (lower bound, other clients add to the rise) |
| 4 | Time-of-day effect (EU mornings) | **Not yet testable** | Hour buckets exist (CET), but 06:00–09:00 has 1 sample and 04:00–07:00 none. Too few samples, and the model mix per hour is not shown |
| 5 | Per-session API cost, plan cost and factor | **Partly** | API $ and plan $ per session are there; the factor only for sessions with at least 1 pp, which is few sessions |
| 6 | Graph of the factor over time | **Yes** | Terminal sparkline over 39 quota rises (12.9x–99.7x); `--json` has `series` and `hours` for outside plots |

## Plausibility checks (by hand)
- **Weekly plan price.** The plan net for 39 pp is $1.4762, so 100 pp is $3.785 a week. Recomputed from your €214.20 a
  year: ÷ 12 ÷ 1.19 VAT = €15.00 net a month, × 1.0935 USD/EUR = $16.40, × 12 / 52 = **$3.785**. Matches.
- **Factors.** $72.95 ÷ $1.4762 = 49.42x, and $74.84 ÷ (0.53 × $3.785) = 37.3x. Both match the report.
- **Buckets.** 1302 showed that the buckets sum exactly to the API total. Here the attributed $72.95 plus the
  unattributed $1.88 gives the $74.84 total.
- **Range end.** The 7d and weekly totals differ by 2 calls and $0.12, because the two ranges end about 10 minutes
  apart. Plausible.
- **Input tokens.** `IN` is tiny (about 2 per call) next to cache reads. That is Claude Code's caching; 1301 checked it
  against transcripts.

## Caveats that limit the answers
- **14 pp of the 53 pp weekly rise is not explained locally.** It comes from other machines or claude.ai web. The
  attributed 49.4x is the local factor; the window's 37.3x is a lower bound for the whole account.
- **Quota steps are 1 pp.** The share per session is spread across the calls inside each rise, so it is an estimate
  between steps. A single session gets a reliable factor only once it moves the quota by several pp.
- **This week mixes Opus, Haiku and canaries.** A factor per model, or per hour by model, would separate a plan effect
  from a model-mix effect. Without it, Q4 can't separate the two.

## Compaction data (for observing 1305–1307 over time)
The owner chose to watch where Claude now compacts (Haiku near 90k, others near 150k) in the data instead of running
more canaries. What the data holds today:

- `compaction_events` has 115 PostCompact rows on 2026-11-27, and 83 of them carry the post-compact size and the model
  (Haiku auto: 75 rows, post-compact 4.6k–36.7k, average 11.6k).
- **Gap 1: no pre-compact size.** The point where compaction fired, which is the number to watch, is not stored:
  PreCompact rows carry no tokens (0 of 115). Claude's `compact_boundary` has `compact_metadata.pre_tokens`; the 1305
  canaries read it from stream files by hand.
- **Gap 2: compaction economics are empty.** All 116 `compaction_economics` rows since 2026-11-26 are
  `insufficient_data`, although the 1303 canary row had economics. Unverified whether this is expected for these
  sessions or a regression.
- The 1302 report does not use the compact-time quota samples from 1303 yet (open LOW in 1303).

## Recommended next steps
1. **Store `pre_tokens` on each compaction row** (from the boundary event or the last main-chain request before it), and
   add a per-model summary of the compaction point to `usage history report`. This turns "observe over time" into one
   command. It is the gap that matters most for 1305–1307.
2. **Check why `compaction_economics` is `insufficient_data`** for every recent row.
3. **Add a factor per model to the report**, and a per-model split of the hour buckets, so Q4 can be tested once
   mornings collect samples. Q4 needs about two more weeks of data with morning work in it.
4. **Add the 5-hour window to Q2**, or decide it is not needed. The weekly window is what limits the plan.

No ticket is filed yet for steps 1–4. Ticket 1300 stays parked.
