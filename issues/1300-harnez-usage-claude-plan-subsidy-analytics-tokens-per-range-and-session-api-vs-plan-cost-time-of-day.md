# 1300 — harnez usage: Claude plan-subsidy analytics (tokens per range and session, API vs plan cost, time of day)

**Status**: Backlog — parked 2026-11-26; gaps go to 1301 (pricing inputs) and 1302 (report); Q2 and Q5 answered by 1308 (closed 2026-11-27: 5-hour share per session, per-session factor for 58 of 126 sessions)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #1215 (cost record per agent job), #375 (quota-history.jsonl), #204 (usage export), #421 (telemetry analytics backlog), #445 (pricing catalog)

---

## Goal

/goal Make `harnez usage` answer the six Claude questions below from local data, or say per question which data is missing and why. Done means each question has a command, or a documented export plus query, and its numbers pass a plausibility check against the raw transcripts and the live quota API. Claude only. Stop and report when a question needs data harnez cannot get (for example usage from other machines), or when a pricing or plan input is the owner's to decide. On a loop or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

The owner wants to know how much the Claude plan subsidises usage compared with API prices, per session and over time. The questions:

1. In/out/cached tokens of the Claude sessions on this machine (a) in the last 7 days and (b) in the last weekly quota window. The window is 7 days but does not start on Monday; take it from the `reset_at` of the weekly window.
2. Plan limit consumed by each session (% of the 5-hour and weekly windows).
3. Ratio of theoretical API cost to plan cost (monthly Claude Pro price) for those ranges.
4. Does the time of day change the ratio? Hypothesis: in EU mornings, while the US sleeps, an EU plan gets more API-equivalent usage per % of the limit.
5. Theoretical API cost and actual plan cost per session, plus the subsidy factor (API cost ÷ plan cost).
6. A graph of the factor over time.

Caveat: the plan's weekly % is shared by every client on the account: other machines (the owner's second machine "esprimo"), claude.ai web usage, and Claude Code here. Local tokens can explain only the local part of a quota delta. Start with the last few days, when the owner says only this machine used the plan (`claude` and `taia` sessions, plus some web usage). The owner will check when esprimo last used the plan.

## 2. Findings (assessment, 2026-11-26, live data on this machine)

What exists:

- `harnez usage --json --agent claude` gives **lifetime** token totals from `~/.claude/stats-cache.json`. It also gives the live 5-hour and weekly `used_percent` and `reset_at` from `api.anthropic.com/api/oauth/usage`, and `plan_tier: "Pro"`. There is no time-range flag, and `cost_usd` is 0.
- `harnez usage --project <dir>` sums Claude transcripts (`~/.claude/projects/*/*.jsonl`, `internal/usage/project.go:107`) per repo, over all time, with no date filter and no per-session rows.
- `~/.local/share/harnez/usage-history/quota-history.jsonl` (#375): 188k lines since 2026-10-28, with integer `used_percent` per window and timestamps. It holds **no tokens**. 481 of the last 2000 lines are exact duplicates.
- `telemetry.sqlite` `turn_token_usage` and `turn_quota_deltas` (#1215) have the right shape: per-turn tokens, `cost_micros`, quota before/after, and `subscription_price_share_micros`. Coverage for Claude is thin. Since 2026-11-19 there are 90 turns in 50 sessions, all from 2026-11-23 on, and `cost_micros` is null in every row. There are only 2 Claude quota-delta rows, both with delta 0. This long host session (bd2185a4) has no rows at all, so these tables seem to cover only some sessions; which ones is unverified.
- `harnez usage export` (#204) writes per-agent token totals and quota % over time. Whether it can give per-session, per-range data is unverified.

Gaps per question:

| Q | Answerable today? | Missing |
|---|---|---|
| 1a 7-day tokens | No command | Date-range sum over transcripts (they carry per-message `usage` and timestamps) |
| 1b weekly window | No command | Window start = weekly `reset_at` − 7d, taken from quota-history; then the same range sum |
| 2 % per session | No | Session time spans joined with quota-history deltas; split when sessions overlap |
| 3 API ÷ plan ratio | No | `spec/pricing.yaml` has only `gpt-5`, with **no Claude model rates** (that is why `cost_micros` is null). The plan price is also wrong, see below |
| 4 time of day | No | Hourly buckets of API-equivalent cost per % of the weekly limit; needs Q2 and Q3 |
| 5 per-session cost and factor | No | Q2 and Q3 per session |
| 6 graph | No | A time series of the factor; `export` JSON or a terminal sparkline |

Data-quality issues found along the way:

- `spec/agent.yaml:13` `plan_prices.claude` is `"Max 5x"`, `100000000` micros ($100/mo). The live API reports `plan_tier: "Pro"`, and the owner says the plan is Pro. There is no `agent.plan_prices` override in `~/.harnez/config.yaml`. The Pro price is not in the repo, so it needs a source or the owner's figure (unverified).
- Claude API prices per model (input, output, cache read, and 5-minute vs 1-hour cache write) must come from Anthropic's pricing page and go in `spec/pricing.yaml` (with a new revision), never as Go defaults (docs/Spec.md).
- Lifetime `input_tokens` (90k) is tiny next to `cache_read_tokens` (3.97G). That fits Claude Code's caching but must be checked against transcripts before it feeds a cost.
- The `used_percent` integers are coarse: 1% of the weekly limit is the smallest step. A per-session share needs many sessions, or a share between steps.
- The duplicate lines in quota-history inflate any count of samples.

## 3. Implementation & Verification Plan

Live-check first: code may have moved since this assessment.

- **M1 (inputs):** add Claude model rates to `spec/pricing.yaml` with their source URL and date. Fix `plan_prices.claude`, or detect the plan from `plan_tier`, at Pro's monthly price, which the owner confirms. Find out why `turn_token_usage.cost_micros` stays null for Claude, and which sessions the per-turn tables cover.
- **M2 (range and session report):** a read-only report, for example `harnez usage history sessions --agent claude --since 7d|--window weekly [--json]`. The name is the developer's to propose. It shows per session: start and end, model mix, in, out, cache-read and cache-write tokens, API cost, share of the weekly and 5-hour %, plan cost (share × weekly plan price, `monthly × 12 / 52` per `spec/agent.yaml:7`) and the factor. It also shows range totals and a note when the quota delta exceeds what local sessions explain (other machines or web).
- **M3 (time of day and graph):** buckets by local hour (factor per hour) and a factor-over-time series: a sparkline in the terminal, plus `export` JSON for outside plots.

Verification: hand-check one session's tokens against its transcript JSONL, and one window's % against quota-history and the live API. Totals of the per-session rows must match the range totals. Run the installed binary on the real data for the owner's "this machine only" days.

Open facts for the owner: the Pro monthly price to use (and currency or VAT); the date esprimo last used the plan; whether web usage should be estimated or only flagged.
