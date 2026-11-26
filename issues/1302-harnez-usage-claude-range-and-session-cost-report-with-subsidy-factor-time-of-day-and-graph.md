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

## Decision (host, sprint start 2026-11-26)

- **Roles.** The developer is `claude:haiku:med`, session `dev-1302`. The host reviews every diff. There is no Searcher: the host refined this ticket against live code today and it cites file:line. The developer's first turn is a read-only plan turn instead.
- **Compaction.** The machine-local `~/.harnez/config.yaml` sets the harnez compact threshold for every Haiku tier to 200000 (owner, 2026-11-26). Claude Code's own `autoCompactWindow` of 200000 is therefore the ceiling (estimated trigger near 167k). The owner said "for now", so the host asks before removing the override at teardown.
- **Prerequisite in M1: share jitter.** `ComputeTurnQuotaShare` / `sameResetMarker` (`internal/usage/turnquota.go:165`, `:222`) must use `turn_quota_pairing.reset_tolerance`, so that the live paired check can produce a share.
- **M1 also aligns the model in the JSON.** `TurnRecord.Model` in the session JSON uses the reported main-chain model when one exists, as the DB rows have done since #1301 M4.
- **Resolved.** The 1301 M2 note "cumulative row still counts cache writes" was fixed in #1301 M3.
- **Plan price.** €15/month net, as above. Label it net, and show gross €17.85 as a second line.
- **Data sources.** Tests use fixture transcripts and quota history under `t.TempDir()`, never the real HOME. Only the host runs the installed binary on real data.

## Plan turn review (host) and M1 pre-work

The developer's plan is accepted with the following decisions and changes.

1. **Command.** New `harnez usage history report [--since 7d | --window weekly] [--session <id>] [--json]`. `harnez stats --agents` stays unchanged. Reason: the report reads analytical history and belongs with `usage history`. Add the command to `docs/CLIDesign.md`.
2. **Hourly factor: not flat.** A flat split (weekly ÷ 168) would only measure activity, not the EU-morning hypothesis. The question is whether one API dollar uses less quota at some hours.
   - Per local hour: factor = API cost in that hour ÷ (quota delta in that hour × the weekly plan price).
   - The hour's quota delta comes from the quota-history rises inside that hour.
   - Show the sample count per hour, and print "n/a" for hours with no quota rise or no cost.
   - This is M2 work. M1 must still store the per-interval data M2 needs.
3. **Reset jitter in quota history too.** Group quota-history samples into windows by `reset_at` within `turn_quota_pairing.reset_tolerance`, the same spec value as #1301 M4. Raw `reset_at` drifts by up to about 0.2 s between samples. Do not add a second tolerance for this.
4. **Transcript layout.** Before you rely on `~/.claude/projects/*/*.jsonl`, check whether Claude Code also writes sub-agent or sidechain transcripts in nested directories. Use only `ls`/`find` on file names and never read real transcript content. Include the nested ones if they exist, deduped by message id across files. Each file's `message.model` sets that call's price, as in `claudeCallPricing`.
5. **Double counting.** Harnez worker sessions run `claude -p` under the same HOME, so their calls are already in the transcripts. The report counts only the transcripts and never adds the `turn_token_usage` rows on top. It may join to those rows only to label a session as a harnez agent session.
6. **The tolerance for "not explained locally"** goes in `spec/usage.yaml` with a schema entry, as you planned. Start at 1 percentage point, because quota history moves in 1% steps.
7. **Plan cost per session** = attributed share × the weekly plan price (net). Totals show net and gross; gross = net × 1.19. Put the VAT rate in spec next to the plan price, never as a Go constant.
8. **Prerequisites as planned.** Apply the reset tolerance in `sameResetMarker` / `ComputeTurnQuotaShare`. Make `rec.Model` use the reported `r.Model`.
9. **Stale Plan lines.** "Keyed by timestamp" means bucketed by timestamp and deduped by message id. The compaction threshold is already 200000. Both are noted here; the Plan text stays as it is.
10. **Commit split.** Commit M1 as two commits, so the review can take them separately:
    - first the prerequisites (`fix(usage): ... (issue 1302 M1)`);
    - then the report (`feat(usage): ... (issue 1302 M1)`).

## M1 review (host)

M1 was delivered in two commits: the prerequisites (6e0406e3) and the range report (62653884).

**What is right**
- The prerequisites are correct. The jitter and fallback-model tests assert real behaviour.
- Pricing matches on real data. Session 2e83cba8 is 2 in, 11422 read, 8764 write and 4 out, which is $0.0019. That equals the hand-checked #1301 M4 canary of 1869 µUSD.
- Nested subagent transcripts are included, and the message-id dedupe works. A 7-day run over 730 MB of transcripts takes 2 s.

**What is wrong (real run, `harnez usage history report --since 7d`)**
- The quota side uses no Claude data at all. The totals are impossible: weekly rise 1213 pp, attributed 808 pp, and a single session showing 79% for $0.48.
- Cause 1: `weeklyGroups` does not filter on `QuotaHistoryEntry.Agent` or `Group`. It reads agy (Antigravity) weekly windows instead, mixing "Claude and GPT models" (12%) with "Gemini Models" (0%). Their `reset_at` is null on both, so they fall into one group, and the series alternates 0↔12.
- Cause 2: Claude's history window name is `"Weekly (7-day)"` (`internal/usage/claude.go:174`, `:240`). `IsWeeklyQuotaWindow` matches only `weekly` / `seven_day`, so no Claude sample is ever used.
- The tests stayed green because their fixtures do not have the real history shape.

## M2 pre-work (do first, before the hourly factor)

1. **Claude only, one series at a time.** Select samples with `agent == "claude"`. Build each series from (agent, group, window) and never mix series. Recognise the Claude weekly window by the name the writer uses: reference the same constant or helper behind `claude.go:174`/`:240`, and never copy the string.
2. **Fixtures in the real shape.** Copy the field shapes from the live file, never its content:
   - `agent`, `group: null`, `window: "Weekly (7-day)"`;
   - `reset_at` drifting between `10:59:59.698` and `11:00:00` across samples;
   - interleaved `Session (5-hour)` and `tokens` rows;
   - interleaved agy rows with null `reset_at` and two groups.
   The test must fail on 62653884.
3. **Invariant tests on Build.** For every window, the sum of session shares plus not-explained equals the rise. A rise is ≤ 100 pp per window. No session's quota % exceeds the window's rise.
4. **A fall that is not a reset** (e.g. 70 → 69 → 71) must not create a 2 pp rise out of 1. Measure from the highest value seen so far in the window, and add a test.
5. **Plausibility check before commit.** Run the installed report on real data, `--since 7d` and `--window weekly`. Quote the totals line and the weekly rise. The rise must be close to the Claude weekly `used_percent` change over the range: the live file reads about 66–71% on 2026-11-26.
6. Then M2 as planned (ticket "Plan turn review" item 2): the per-local-hour factor with its sample count, "n/a" for hours without data, a sparkline, and `series` in `--json`.

Owner decision 2026-11-26: the `~/.harnez/config.yaml` Haiku 200k override **stays** after the sprint.

## M2 review (host)

M2 was delivered in two commits: the Claude-only fix (acfa193e) and the hourly factor (7b95c969). The red-first tests are real. The old tests changed only in the fixture helper's window name, and no assertion was removed.

**Real data, host run of the installed binary**
- `--window weekly` gives a rise of 34 pp. Quota history goes 37 → 71 in the current window (reset 2026-11-28 11:00Z), so the rise checks out.
- `--since 7d` gives a rise of 53 pp. The extra 19 pp is from 2026-11-19 to 11-21, for example 67 → 76 on 11-20.
- There are no local calls in that period: only one transcript file was last modified then. The report rightly lists those 19 pp as "not explained locally", probably the second machine or web use. The owner checks esprimo's online times.

**What is still wrong: attribution only sees calls in the ~30 s before a rise**
- Each `Interval` runs from the sample just before a rise to the rising sample, e.g. 10:05:33 → 10:06:00. Calls made during flat stretches, which built up the next 1% step, fall into no interval.
- Result: only 24 of 34 pp are attributed. The total factor is $37.09 of *all* calls ÷ the plan cost of the 24 attributed pp, which is 40.8x. That mixes two bases.
- Per-session factors jump from 0.08x to 63x because of where a 1% step happened to land. One example: session f8441a87 gets 1 pp for $0.003.

## M3 pre-work (attribution fix, then close-out)

1. **Interval = from the previous rise point to this rise.** Within one window, interval k runs from the time the value last rose, or the window start or range start for the first, to the sample where it rises again. Every call in the window then falls into exactly one interval. A rise with no priced call in its interval stays "not explained locally".
   - Test: calls spread over a flat stretch get the rise.
   - Test: the sum of attributed call costs equals the total API cost of calls inside closed intervals.
2. **Calls after the last rise belong to an open interval.** List them as "pending (no quota step yet)" with their API cost. They get no quota share and must not count in the factor.
3. **Totals use one base.** Total factor = API cost of attributed calls ÷ plan cost of attributed pp.
   - Also print a window line: API cost of all local calls ÷ (window rise × weekly plan). Label it "upper bound on local quota" or similar, since other machines add to the rise.
   - Test that the parts add up.
4. **Per-session factor noise.** A session with less than 1 pp attributed has a factor driven by the 1% step size. Print its factor as "n/a (< 1 pp)", or mark it, and say so in the legend. Set the threshold in `spec/usage.yaml` with a schema entry, starting at 1 pp.
5. **Hours.** Keep a call's share in the call's local hour, as M2 does. After item 1 the hour table covers all attributed calls. The SAMPLES column counts attributed rises with a share in that hour.
6. **Plausibility before commit**, quoting only aggregate lines:
   - `--window weekly`: attributed pp plus not explained equals the rise of 34 pp or more.
   - Attributed API cost plus pending API cost equals the total API cost.
   - Quote the totals line, the window line and the hour table.
7. **Docs.** Add the report's columns and the meaning of attribution, pending, "not explained locally" and the factor bases to `docs/Telemetry.md`, next to the M1 text.
8. **Then gates.** Run `HTO=0 make test-q1` to `scratchpad/q1-1302.log` and quote the final summary line. Do not close the ticket; the host closes it after the long Haiku session check.

## Decision (decide mode, 2026-11-26)

- **Review.** After M3 (attribution fix) is delivered, a one-shot `agy:neon:med` reviewer covers the whole #1302 range 6e0406e3..HEAD, about 1.7k lines. This follows the owner's rule: neon:med as one-shot reviewer for bigger code chunks. neon:med is on the separate agy plan and needs no `--accept-cost` (dry-run checked). Its findings become M4 pre-work. If it finds nothing at MED or above, the host closes.
- **Long Haiku session.** dev-1302 is the long Haiku session. At close, record its turns, peak input, the first turn over 100k and the cost per tier.
- **Dev model.** dev-1302 stays on `claude:haiku:med`. The work is well-specified report logic, and med has delivered red-first tests every round.

## M3 review (host and agy:neon:med reviewer)

M3 was delivered in 77318286 (attribution) and c2c75eaa (docs). q1 is green: 82 packages ok, no `--- FAIL`.

**Real data, current weekly window**
- 25 pp are attributed, with $37.80 attributed and $0.34 pending: factor 39.95x.
- The window line is 35 pp for $38.14: 28.79x.
- 10 pp are not explained.
- The parts add up.

**Host finding: a quiet rise gets pinned on one tiny call.** Session f8441a87 has one 1 pp rise from 2026-11-24 19:30 to 11-25 09:36 against $0.003 of local cost, a factor of 0.08x. Session 25bf9316 is similar at 0.05x. The rise almost surely came from another machine or the web.

**Reviewer findings (one-shot `agy:neon:med`), checked by the host against the code**
- HIGH, confirmed: the first interval of a window starts at `reset_at - 7d` (`windowStart`, `report.go:464`), even when the first sample is a mid-window baseline such as 37%. Calls made before the baseline sample are swept into the first rise, which measures only the change from the baseline.
- HIGH → MED: a nil `reset_at` on Claude weekly rows merges windows across resets, or splits one window into overlapping groups (`report.go:427`). The live file has no Claude weekly row with a nil `reset_at`, so this is a guard for later, not a live bug.
- MED: a session that mixes priced and unpriced calls gets nil API and pending cost, but a factor computed from the priced subset (`report.go:166`, `:204`).
- MED: calls exactly at `iv.Start == Since`, and calls after an older window's last rise under `--since`, land in no interval and are mislabelled "pending (after the last quota rise)" (`report.go:134`, `:208`, `:361`).
- MED: `ScanCalls` reads every `.jsonl` file whole, with no `since` filter (`scan.go:44`). `ParseClaudeTranscriptCalls` never checks `sc.Err()`, so a line over 16 MiB silently drops the rest of the file (`transcript.go:41`).
- MED, test gaps:
  - the `RisePct > 100` check right after `== 11` is vacuous and sums over windows (`quota_shape_test.go:91`);
  - no test covers the `cache_creation` TTL split in a JSONL fixture;
  - no test runs `ScanCalls` → `Build` end to end;
  - no test covers the CLI flags or wiring;
  - no test covers the new spec loaders.
- LOW: `usage.IsWeeklyQuotaWindow` has no callers. `ClaudeTranscriptCall.Model` duplicates `Usage.Model`.
- LOW: the docs (`Telemetry.md:270`) and comment (`report.go:128`) say intervals are clamped to `Since`, but they are not. Under `--session`, `Hours`, `Series` and `Window` stay unfiltered.
- LOW: `sessionFactor` prints `n/a (< 1 pp)` when the plan price is unknown (`render.go:139`).

## M4 pre-work (review fixes, then close-out)

1. **Baseline.** A window's first interval starts at its first sample, not at `reset_at - 7d`. Exception: the first sample reads 0%, so nothing was used before it. Calls between the window start and the first sample go to a "before first quota sample" bucket: API cost shown, no share, not in the factor. Test both cases.
2. **Nil `reset_at`.** Skip Claude weekly rows with a nil `reset_at`, and count them in a "skipped samples" note. Test that they never merge or split windows.
3. **Every in-range call lands in exactly one bucket:** an interval, pending (after the newest window's last rise), "before first quota sample", or "after the window's last rise" (older windows under `--since`). Use the half-open `[Start, End)` consistently and clamp the interval start to `Since`. Invariant test: the bucket costs sum to the total API cost, including at the boundaries.
4. **Mixed pricing.** A session with any unpriced call shows its factor as n/a, consistent with its nil API cost. The unpriced call count goes in a note.
5. **Quiet rises.** For each rise, compute local cost ÷ (rise × weekly plan). When that is below `usage_report.min_local_factor` in `spec/usage.yaml` (with a schema entry), count the rise as "not explained locally" and give it no share.
   - The default value must come from data. Print the per-rise factor distribution from the real run (aggregates only: min, p10, median), set the default well below p10, and say why in the commit message.
   - Test it on the f8441a87 shape: 1 pp, $0.003, a 14-hour gap.
6. **Scan.** Skip a transcript file whose mtime is before `Since` (it cannot hold newer calls), check `sc.Err()`, and return or report an error for over-long lines. Test the mtime skip.
7. **Tests.** Fix the vacuous check: assert ≤ 100 per window. Add:
   - a JSONL fixture with the `cache_creation` 5m/1h split;
   - a `ScanCalls` → `Build` end-to-end test on a temp dir;
   - a CLI test for `--since`, `--window weekly`, `--session` and `--json`;
   - tests for the new spec loaders.
8. **LOW items.** Remove `IsWeeklyQuotaWindow` and the duplicate `Model` field. Make the docs and comments match the code. Under `--session`, either filter `Hours`/`Series` or say in the output that they are for all sessions. Print a plain `n/a` when the plan price is unknown.
9. **Gates.** `make install`. Re-run the real report (aggregates only) and check that the bucket sums add up. Then `HTO=0 make test-q1 > scratchpad/q1-1302-m4.log 2>&1`, grep for `--- FAIL` and give the ok count.

## M4 review (host)

M4 delivered (review fixes): 37914ef4 red tests, b74d02fc timeline spans and half-open buckets, 8e9180fc `usage_report.min_local_factor`. Dev q1: 82 ok, 0 `--- FAIL`.

Host checks on the installed binary:
- Buckets sum to the total API cost exactly: weekly window $40.0066 = 39.4009 attributed + 0.0051 quiet + 0.0399 before first + 0 after older + 0.5607 pending. The same holds for `--since 7d`.
- 90 days: the bucket sum is 668.92 USD; totals are n/a because one session has unpriced calls (expected, M4 item 4).
- Every changed assertion in the existing tests is the same or stronger. `TestBuildEveryCallInExactlyOneBucket` checks every bucket by value, and `TestBuildHalfOpenBoundaryGoesToNextBucket` checks the boundary.

Findings:
1. **The default `min_local_factor` 0.1 is a near miss.** The weekly report still attributes a 1 pp rise at 09:00 for $0.0038, factor 0.1004x, which is the series minimum. The 90-day series below 20x is: 0.10, 0.16, 0.19, 1.1, 3.29, 3.55, then 11.01 and up (plus 0.05 and 0.08, already quiet). The rises at 0.10 to 0.19x have the same shape as f8441a87.
2. **LOW: two meanings of "pending".** The totals row `PENDING $` is API − ATTRIB ($0.6472 in the weekly window), but the "not in a quota rise" line calls only $0.6022 pending. The rest is the quiet and before-first buckets.

## Decision (decide mode, M4 review)

- `min_local_factor` default 1.0. Reason: below 1x, the local API cost is less than the plan cost of the same quota, which cannot drive a rise when the median is 61x. On the 90-day data it is the widest gap below 10x (0.19 → 1.1). It flags 5 rises and leaves 1.1x and up attributed.

## M5 pre-work

1. Set `usage_report.min_local_factor: 1` in `spec/usage.yaml`. Rewrite its comment with the measured series above (0.05, 0.08, 0.10, 0.16, 0.19 quiet; next 1.1). Update `TestReportMinLocalFactorFromSpec` and `docs/Telemetry.md`.
2. Add a test: a rise at factor 0.1004 with default options is quiet. Use the real shape: 1 pp, $0.0038 local, weekly plan 3,785,192 µUSD. Also add a near-miss test: a rise at 1.1x stays attributed.
3. Rename the totals column `PENDING $` to `UNATTRIB $` (API − ATTRIB). Also rename its JSON field, and keep the bucket line's "pending" for the open tail only. Update the legend and `docs/Telemetry.md`. The session rows use the same column.
4. Gates: `make install`, the real weekly report (aggregates only), and `HTO=0 make test-q1`.
