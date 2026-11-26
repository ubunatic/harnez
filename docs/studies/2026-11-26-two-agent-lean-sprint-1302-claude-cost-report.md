---
kind: sprint
date: 2026-11-26
ticket: 1302
branch: sprint/1302
models:
  searcher: skipped # ticket written by the host against live code (1300 assessment, 1301)
  developer: claude:haiku:med
  reviewer: agy:neon:med # one-shot over M1..M3; the host reviewed every milestone
---

# Two-Agent Lean Sprint: #1302 — Claude cost report

## Context

#1301 added per-call Claude pricing and quota capture. #1302 adds `harnez usage history report`. The report gives the API cost against the Pro plan cost per session and per range, the subsidy factor, a factor by local hour and the factor over time. The owner pays €180 net + 19% VAT a year.

## Milestones

| Milestone | Commits | Result |
|---|---|---|
| Fix pairing jitter | 6e0406e3 | `ComputeTurnQuotaShare` tolerates reset_at jitter; rows keep the model |
| M1 report | 62653884 | transcript scan, per-call pricing, weekly-rise attribution, plan net and gross |
| M2 real-data fixes | acfa193e, 7b95c969 | Claude-only "Weekly (7-day)" window, rises from the peak; hourly factor, sparkline |
| M3 attribution | 77318286, c2c75eaa | intervals run from the previous rise; pending; window line; 1 pp factor threshold; docs |
| M4 review fixes | 37914ef4, b74d02fc, 8e9180fc | timeline spans with half-open buckets, every call in one bucket; nil reset_at skipped; mixed pricing → n/a; mtime skip and scan errors; `min_local_factor` |
| M5 | 1ab07191, 4db667a5 | `min_local_factor` 1 from the measured gap; `PENDING $` → `UNATTRIB $` |

## Verification

- Real weekly window (2026-11-21 → 11-26): API $40.51, attributed $39.40 for 23 pp, factor 45x. The window line (all local calls over the whole 36 pp rise) gives 29.7x as an upper bound. 13 pp have no local call: other machines or web use.
- The buckets (attributed, quiet, before first sample, after older window, pending) sum exactly to the API total on the weekly, 7-day and 90-day runs.
- API prices check against the #1301 canary: session 2e83cba8 costs $0.0019, and the canary measured 1869 µUSD.
- Live `paired` turn-quota outcomes now exist (6 rows, 4 of them from dev-1302). #1301 had left this open.
- Developer q1 after M3, M4 and M5: 82 ok, 0 `--- FAIL`.

## The long Haiku session (dev-1302, haiku:med, compact threshold 200k)

- 12 prompts, 204 API calls, 4 compactions, peak context 166.7k tokens.
- The first call over 100k came at 12:08 UTC, in M2. After that, 107 of 204 calls ran in the >100k tier.
- API cost $1.73: $0.20 for the 97 calls under 100k, $1.53 for the 107 over it. The 5x tier is 88% of the cost for 52% of the calls.
- Quota: 0.88 pp of the weekly limit attributed to it.
- Lesson: with a 200k compact threshold, a long med session spends most of its money above 100k. The 80k default would have kept almost all of it in the cheap tier, at the cost of more compactions.

## Observations

- **Real runs found what tests hid, at every milestone.** M1's first run showed a 1213 pp rise: agy rows were mixed in, and the Claude weekly window name was never matched. M3's run pinned a quiet 1 pp overnight rise on a $0.003 call.
- **The developer's chosen default was a near miss.** It set `min_local_factor` from the distribution summary (p10 29x) to 0.1. In the real weekly report, a rise at 0.1004x still passed. Reading the low tail itself (0.05 … 0.19, then 1.1) gave a clear gap. Check a threshold against the values next to it, not against a percentile.
- **The neon:med one-shot review paid off.** Over M1..M3 it found a real HIGH: the first interval started at the window start despite a mid-window baseline. It also found calls that landed in no bucket, and test gaps. The host's diff reviews had passed all of these. One of its HIGHs (nil reset_at) was a guard only: there are no such live rows.
- **haiku:med held up on complete pre-work.** Each milestone matched its pre-work list. It flagged its own deviations: the tests were compile-red rather than behaviour-red at the old base, and one commit message used loose wording.

## Follow-ups

- #1303: a quota sample and the context size at each compaction, which gives the report more boundaries.
- The 13 pp not explained locally this week are probably the second machine (esprimo) or web use. The owner is checking.
