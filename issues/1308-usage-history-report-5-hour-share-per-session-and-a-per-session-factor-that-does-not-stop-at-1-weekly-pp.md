# 1308 — usage history report: 5-hour share per session and a per-session factor that does not stop at 1 weekly pp

**Status**: In Progress — M0 delivered (host), M1 next
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #1300 (questions 2 and 5), #1302 (report, attribution), docs/studies/2026-11-27-claude-plan-subsidy-analytics-assessment.md

---

## Goal

/goal Make `harnez usage history report` answer #1300 Q2 (share of the 5-hour **and** weekly limits per session) and Q5 (API cost, plan cost and subsidy factor per session) for most sessions, not only the few that move the weekly quota by 1 pp. Done means: a 5-hour share column per session, a per-session factor for every session above a spec threshold measured in 5-hour pp, each factor marked as measured (solo rises) or split (shared rises), a per-model rollup, and live numbers that pass the invariants and hand checks below. Stop and report when the 5-hour data cannot support it (M0), or when a design choice is the owner's. On a loop or no progress, clear the goal and write a failure report.

## Problem (live data, 2026-11-27, weekly window 2026-11-21 11:00 .. 2026-11-27 11:59 UTC)

- **Q2:** the report shows only the weekly share. The 5-hour window is collected (`claude` `Session (5-hour)` rows in quota-history, as many samples as the weekly window) but not used.
- **Q5:** 120 sessions, only 5 have ≥ 1 weekly pp, so 115 show `n/a (< 1 pp)` (`usage_report.min_session_quota_pct: 1`, spec/usage.yaml). One host session holds 19.4 of 40 attributed pp.
- **The split hides the session.** `attribute` (internal/usagereport/report.go:438) splits a rise by API cost across the calls in its interval. So a session's factor is the cost-weighted average of the rise factors it took part in; it says nothing about the session itself unless the session ran alone during those rises.

Rough count by the host (unverified, M0 measures it): over the window the 5-hour samples rose about 698 pp in 308 up-steps against about 91 pp in 55 up-steps for the weekly window (raw positive deltas, no flicker filter; the report's weekly rise is 53 pp). So one 5-hour pp is roughly 8-13 times finer than one weekly pp, and 5-hour steps have fewer overlapping sessions.

## Plan

### M0 — read-only measurement (no code)
1. With the report's own sample handling (dedupe, reset tolerance, flicker), measure for the weekly window: 5-hour rise, weekly rise, their ratio overall and per 5-hour window (does the ratio vary by window or by hour?), up-step count, and how many 5-hour intervals contain calls from exactly one session.
2. Verdict: is a 5-hour-based share precise enough to give a factor for sessions below 1 weekly pp? If the ratio varies a lot between 5-hour windows, say so; that changes M2.

### M0 delivered (host, 2026-11-27, quota-history aggregates only, no transcripts)
Paired Claude 5-hour and weekly samples from 2026-11-21 11:00 UTC, split at 5-hour resets (drop > 5 pp), running max per window:

| 5h window (UTC) | 5h rise | weekly rise | ratio |
|---|---|---|---|
| 11-23 07:07-12:45 | 100 | 44 | 2.3 (anomaly, see below) |
| 11-23 12:50-17:58 | 66 | 4 | 16.5 |
| 11-23 18:00-11-24 15:59 | 100 | 6 | 16.7 |
| 11-24 16:04-21:08 | 100 | 7 | 14.3 |
| 11-24 21:13-11-25 12:39 | 18 | 1 | 18.0 |
| 11-25 12:44-11-26 02:58 | 28 | 3 | 9.3 |
| 11-26 03:03-12:09 | 46 | 5 | 9.2 |
| 11-26 12:13-17:06 | 100 | 9 | 11.1 |
| 11-26 17:11-22:16 | 22 | 1 | 22.0 |
| 11-26 22:21-11-27 03:39 | 58 | 6 | 9.7 |
| 11-27 03:44-13:46 | 37 | 4 | 9.2 |
| 11-27 13:51-14:48 | 12 | 1 | 12.0 |

- Total 687 / 91 = 7.55, but without the first window 587 / 47 = **12.5**. With weekly rise ≥ 4 pp per window the ratio is 9.2-16.7. One 5-hour pp is about 1/12 of a weekly pp, so a session needs about 0.08 weekly pp, not 1, for one 5-hour step.
- The first window's 44 weekly pp are an artefact of this rough count (no samples 11-21 11:00 .. 11-23 07:07; the weekly value jumps from the baseline). The report's own handling (first sample as baseline, #1302 M5) must be used; the developer checks this case in a fixture.
- Four windows hit 100 % (the 5-hour limit bound). Those windows are capped: usage beyond the cap was blocked, so the 5-hour rise there is a lower bound.
- Windows with 1 weekly pp give ratios 12-22: quantization noise.

**M0 verdict (host decision):** precise enough. M2 converts with the **range ratio** = Σ 5-hour rise ÷ Σ weekly rise over the 5-hour windows in range whose weekly rise is at least `usage_report.min_ratio_weekly_pct` (new spec key, start 4), never per window (too noisy at 1-4 weekly pp). Print the ratio and the window count used. Start `min_session_five_hour_pct` at 1 (one 5-hour step). The solo-interval count is measured after M1 on the installed binary.

### Plan turn (dev-1308, 1m9s, peak 64.6k) and host decisions
Plan accepted: parameterise the window selector (`isClaudeWeekly`, `windowStart`, `weeklyGroups`, `buildTimeline`) instead of copying; M1 and M2 as one commit each.

**Pre-Work / Required Refinements for M1-M2 (host decisions, decide mode):**
1. **Weekly delta per 5-hour window** = weekly value at this window's last sample minus the weekly value at the previous window's last sample (contiguous partition, the same monotone/flicker handling as the weekly path). The per-window deltas then sum to the range's weekly rise; test that.
2. **Capped windows count in the ratio.** At 100 % Claude refuses further calls, so no usage is hidden; the 5-hour rise there is exact. No lower-bound flag.
3. **Quiet check on 5-hour rises** uses weekly-equivalent pp (5-hour pp ÷ ratio) against `min_local_factor`. When the ratio is n/a, the 5-hour quiet check is off and the output says so; rises are not marked quiet by default.
4. **Sessions with ≥ 1 weekly pp** keep the weekly factor and show the 5-hour factor beside it.
5. **Acceptance restated:** Σ weekly-equivalent pp vs attributed weekly pp is a reported gap (print both and the difference), not an identity.
6. M0 is re-measured on the installed binary after M2 with the code's definition; the host records it.

### M1 — 5-hour share per session (Q2)
- Run the same interval attribution on the Claude 5-hour window: resets every 5 hours are expected, not errors. Reuse the weekly code path; do not copy it.
- Per session: `QUOTA 5H %` (sum of 5-hour pp) next to the weekly `QUOTA %`; JSON field next to `quota_pct`. Totals: 5-hour rise, attributed, not explained locally, per the same rules as the weekly window (quiet rises, before first sample, pending).
- Invariants, tested: per 5-hour window, session shares plus not explained equal the rise; no rise above 100 pp per 5-hour window.

### M2 — per-session factor that works below 1 weekly pp (Q5)
- Weekly-equivalent share of a session = its 5-hour share × (weekly rise ÷ 5-hour rise) **measured over the same range or per 5-hour window** (M0 decides which). Never a constant. Print the ratio used.
- Show a session's factor when its 5-hour share is at least `usage_report.min_session_five_hour_pct` (new spec key with schema entry; start value from M0). Keep `n/a` with the reason below it.
- Mark each factor `measured` when at least half its share comes from rises where it was the only session with calls, else `split`. Legend line explains both.
- Per-model rollup rows (API cost, weekly-equivalent pp, plan cost, factor), and the factor from solo rises only per model, so Haiku and Opus can be compared.

### M3 — docs and live acceptance
- `docs/Telemetry.md` (report section from #1302): 5-hour column, weekly-equivalent share, measured/split, per-model rows.
- Live `--window weekly` and `--since 7d` runs on the installed binary.

## Acceptance
- Weekly invariants from #1302 still hold (buckets sum to total API cost; attributed + not explained = weekly rise).
- 5-hour invariants above hold on live data.
- Sessions with a factor on the live weekly window: well above today's 5 (state the number); each has measured/split.
- Hand check of two sessions: one measured, one split, from the raw interval list.
- Sum of weekly-equivalent pp over sessions is within the window's attributed weekly pp ± the tolerance M0 states.

## Constraints
- Read-only report; no new collection. Claude only.
- New values in `spec/usage.yaml` with schema, never Go defaults.
- Tests use fixtures; never read real transcripts or real usage data into tests or reviewer prompts.

## Decision
- 2026-11-27 (host, decide mode): one ticket for Q2 and Q5, because the 5-hour share is what makes the per-session factor finer. Lean sprint, developer claude:haiku:med and final one-shot review agy:neon:low, the owner's picks for 1305.
