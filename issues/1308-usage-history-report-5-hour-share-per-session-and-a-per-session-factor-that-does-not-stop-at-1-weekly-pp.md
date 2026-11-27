# 1308 — usage history report: 5-hour share per session and a per-session factor that does not stop at 1 weekly pp

**Status**: In Progress — M3 delivered, acceptance passed, legend wording (M3b) left
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

### M1 delivered: 5-hour share per session (84a0712c)
- Window selector (`weeklySel`/`fiveHourSel`) through one timeline path; `QUOTA 5H %` column, `five_hour_quota_pct`, `five_hour_rise_pct`, `five_hour_not_explained_pct`, `five_hour_intervals`; tests in `internal/usagereport/fiveh_test.go`. Dev: one `make test-q1`, 83 ok, 0 FAIL.
- Host live check (installed binary, `--window weekly`): 303 5-hour rises, 661 pp; Σ session 5-hour shares 465 + not explained 196 = 661 (exact). Weekly 55 pp, 41 attributed, 14 not explained (14/55 = 25 %, 196/661 = 30 %: same order, as expected for account-wide windows). Rough ratio 661/55 = 12.0, M0 said 12.5. 59 of 123 sessions have ≥ 1 5-hour pp (today 5 have ≥ 1 weekly pp).
- Implausible, expected until M2: with the quiet check off, session 25bf9316 (1 call, $0.0019) takes a whole 2.00 5-hour pp rise.

**Pre-Work for M2 (host):**
1. Quiet check on 5-hour rises via the ratio (decision 3). Regression test with the 25bf9316 shape: 2 5-hour pp, $0.0019 local, ratio 12, weekly plan 3,785,192 µUSD ⇒ 0.167 weekly-eq pp, plan $0.0063, factor 0.30 < `min_local_factor` 1 ⇒ quiet, no share. Near miss: the same rise at $0.0070 (factor 1.1) stays attributed.
2. The per-window weekly-delta sum test deferred from M1 (decision 1).
3. The text summary line under the table states the ratio, the number of 5-hour windows it used and `min_ratio_weekly_pct`; without a ratio it says why.
4. Review: to spare the Claude weekly quota (92 % at 15:50 CET, resets 2026-11-28 11:00 UTC), no per-milestone Claude reviewer. One agy:neon:low one-shot review covers M1+M2 (separate agy plan). If the Claude weekly quota reaches 97 %, the host pauses the sprint.

### M2 delivered: ratio, weekly-equivalent pp, 5-hour factor, models (251237c0)
- Spec keys `min_ratio_weekly_pct` (4) and `min_session_five_hour_pct` (1) with schema; `fiveh.go` (window partition, range ratio, quiet check via weekly-eq pp, solo shares, `ModelRow`); `5H W-EQ %` and `5H FACTOR` columns with measured/split; summary and legend lines; `ratio_test.go`. Dev: 83 ok, 0 FAIL, but several package runs before `make test-q1` (over the one-run budget).
- Host live check (installed binary, `--window weekly`): quiet fix works (25bf9316 now n/a, $0.0019 unattributed). 57 of 123 sessions have a 5-hour factor (22 measured), against 5 with a weekly factor. Σ session 5-hour shares 466 + not explained 204 = 670 (exact). Weekly invariants unchanged (55 rise, 41 attributed, 14 not explained).
- **Implausible: range ratio 6.88 from 8 windows**, against M0 12.5 and raw 670 / 55 = 12.2. So Σ weekly-eq is 67.7 pp against 41 pp weekly attributed, and every 5-hour factor is about 1.75 times too low (Haiku 14.3x, Opus 43.3x). Cause: the 8 windows match M0's 7 windows with ≥ 4 weekly pp plus the anomalous first window. In `fiveHourWindows` (fiveh.go:89), when the weekly group changes between `prevAt` and a window's last sample, `weekly = v` (the new group's peak from 0). The weekly report instead takes the new group's first sample as the baseline (#1302 M5). The first in-range window (the weekly reset on 11-21 11:00, first sample on 11-23 07:07) so gets the whole pre-sample weekly level (about 38-44 pp) as its rise.

**Pre-Work for M2b (fix before M3):**
1. On a weekly-group change, the window's weekly rise = the peak at its last sample minus the new group's first-sample value, which is the same baseline as the weekly timeline. Apply the same rule to the 5-hour pp of the window that holds the first sample, if `rises` does not already exclude the pre-sample level. Reproduction test first: a weekly reset, then a sample gap, then a new group whose first weekly sample is 40 % and whose 5-hour rises are small. The ratio must not count the 40.
2. The partition-sum test from M1 pre-work 1, still missing: over in-range windows, Σ window weekly rises = `weekly_rise_pct` of the same report, including the reset-gap fixture from item 1.
3. Expected live result after the fix: ratio about 12 (M0 12.5; raw 12.2), Σ weekly-eq near 41 pp, and the gap printed. The host checks this; the developer does not run the live report.
4. One `make test-q1` only; no package runs before it.

### M2b delivered: weekly baseline on a weekly reset (899e358c)
- `weeklyPath.start` holds each weekly group's first reading; on a group change the window's weekly rise is the peak minus that baseline. Tests `weekly_gap_test.go`: the reset-gap fixture (failed before: 41 pp instead of 1) and the partition sum Σ window weekly rises = `rise_pct`. Dev: one `make test-q1`, 83 ok, 0 FAIL.
- Host live check (`--window weekly`, 15:xx UTC): **ratio 12.19** from 8 windows (M0 12.5, raw 677 / 56 = 12.1). Σ weekly-eq 38.97 pp vs 42 pp weekly attributed: **gap −3.03 pp (−7 %)**. 58 of 123 sessions have a 5-hour factor (23 measured). Models: Haiku 19.4 weekly-eq pp, factor 25.0x (96 solo rises); Opus 19.5 pp, 76.0x (52 solo rises). Cost-weighted mix ≈ 50x, matching the weekly attributed factor (≈ 49x). 25bf9316 stays quiet.
- Review: one agy:neon:low over 9b0dd647..899e358c is running; its findings land below as M3 pre-work.

**Review (agy:neon:low, 2m10s, 9b0dd647..899e358c) and host triage. Pre-Work for M3:**
1. **MED (host: valid, was HIGH):** fiveh.go:110-123. When a 5-hour window spans a weekly reset, `v - path.start[gi]` drops the old group's rise from `prevAt` to that group's last sample. The weekly report counts it, so Σ window weekly ≠ `rise_pct` for a range that holds the reset (`--since 7d`). Fix: add the old group's peak at its last sample minus `pv`. Extend the partition-sum test with a reset inside one 5-hour window, where the old group still rises before the reset.
2. **MED:** fiveh.go:106-121. When the first weekly sample is after `wins[0].first` (`pok` false), take the weekly group's first reading as the baseline instead of leaving `weeklyOK` false. Test it.
3. **HIGH (test gap; the plan promised it):** test `min_session_five_hour_pct` at exactly 1 5-hour pp (factor shown) and just below (factor nil, `n/a` in the text with its reason).
4. **MED (test gap):** an end-to-end `Build` + `RenderText` test with a `split` basis (two sessions in one rise), the weekly `FactorBasis`, and a model with only shared rises (`ModelRow.Factor` nil).
5. **LOW:** a capped 100 % window counted in `fiveHourRatio` (decision 2). Fix the stale comments at report.go:170-171 (claims no 5-hour quiet check) and fiveh.go:91-92 (claims "new peak alone").
6. **Host decision, no code:** range-edge windows. The ratio uses whole 5-hour windows whose last sample lies in Since..Until. A window across `Since` brings its pre-`Since` pp into the ratio, and the open window after `Until` is left out. That is fine for a conversion rate (whole windows, never cut); M3 documents it in Telemetry.md.
7. **M3 docs:** extend `docs/Telemetry.md` "Claude Range Report" (line 259; root evergreen, not a copy). Cover the `QUOTA 5H %`, `5H W-EQ %` and `5H FACTOR` columns; measured/split; the range ratio and its window rule (item 6); `min_ratio_weekly_pct` and `min_session_five_hour_pct`; the 5-hour quiet check via weekly-eq pp; the model rows (factor from solo rises only); the reported gap between Σ weekly-eq and weekly attributed. Numbers in docs only as an example marked with a date.
8. One `make test-q1`, then `make install`. The host runs the live acceptance and closes the ticket.

### M3 delivered: reset-span fix, tests, Telemetry.md (475cbff6, 47df949a)
- `riseBetween` sums the weekly rise across groups (the old group up to its last peak, then each new group from its first reading); a late first weekly sample uses the group's first reading. New tests: reset inside one 5-hour window (3 → 5), late first sample, factor gate at exactly 1 pp and below, end-to-end split basis with a shared-only model. Host rerun of the package: all 20 5-hour/basis tests pass. The capped-window ratio test (review item 5) was not added; decision 2 is documented instead.
- **Live acceptance (host, installed binary, 2026-11-27 ~16:30 CET):**
  - `--window weekly` and `--since 7d` give the same output: history starts at the weekly reset (11-21 11:00 UTC), so 7d holds no older samples.
  - Weekly invariants: buckets $79.568006 + quiet $0.008888 + before first sample $0.039859 + pending $0.231701 = API $79.848454 (exact); 43 attributed + 14 not explained = 57 rise.
  - 5-hour invariants: Σ session shares 485 + not explained 203 = 688 pp (exact), over 315 rises.
  - Range ratio **11.98** from 9 windows (M0 12.5). Σ weekly-eq **40.48 pp vs 43 pp** weekly attributed: gap −2.52 pp (−5.9 %).
  - **58 of 126 sessions** have a 5-hour factor (22 measured, 36 split); 5 have a weekly factor.
  - Models (solo rises only): Haiku 24.6x (96 solo rises, 20.0 weekly-eq pp); Opus 74.7x (52 solo rises, 20.5 pp); Sonnet n/a (no solo rise).
  - Hand check, measured: the session with the largest measured share has 32.68 5-hour pp, ÷ 11.98 = 2.727 weekly-eq pp, plan $0.1032; API $3.0998 ⇒ 30.03x, exactly as reported. Its weekly factor is 47.5x on 1.72 weekly pp (1-pp steps).
  - Hand check, split: the host session has 164.87 5-hour pp ⇒ 13.76 weekly-eq pp. The reported 82.4x uses the cost of its calls inside attributed 5-hour rises (about $42.9), not its API $44.63 (which would give 85.7x). Its weekly factor is 53.3x on 22.0 pp. Split factors carry the cost-split error; measured ones are the ones to trust.

**Pre-Work for M3b (last turn):**
1. LOW, wording: the legend (render.go:69) and docs/Telemetry.md:321 say `5H FACTOR = API $ / plan net`, but the code (fiveh.go:288) uses the cost of the session's calls in attributed 5-hour rises. Say that in both places. No code change.

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
