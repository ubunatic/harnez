# 1301 — Claude cost inputs for usage analytics: API rates, Pro plan price, per-turn coverage, live canaries

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #1300 (parked analytics, its §2 assessment), #1302 (report, depends on this), #1215 (per-job cost record), #445 (pricing catalog), #375 (quota-history)

---

## Goal

/goal Make every Claude turn that harnez records carry a correct API cost and plan-share cost. Prove it with live canaries. Done means:
- `spec/pricing.yaml` has sourced Claude rates.
- The Claude plan price matches the owner's Pro plan.
- `turn_token_usage.cost_micros` and `turn_quota_deltas.subscription_price_share_micros` are filled for the canary runs and match a hand calculation.
- The tickets record which Claude sessions the per-turn tables cover, and why.

Stop and report when a price needs the owner's figure, or when a canary would go past the run budget below. On a loop or no progress, clear the goal and write a failure report.

## Problem

The #1300 assessment (2026-11-26) found:
- `spec/pricing.yaml` has only `gpt-5` and no Claude model rates, so `cost_micros` is null on every Claude turn.
- `spec/agent.yaml:13` prices the Claude plan as "Max 5x" at $100, but the live API reports `plan_tier: "Pro"`.
- Since 2026-11-19 the per-turn tables hold 90 Claude turns in 50 sessions and only 2 Claude quota-delta rows (both 0). The long host session bd2185a4 has no rows.
- 481 of the last 2000 lines in `quota-history.jsonl` are exact duplicates.

Re-check all of this against live code and data first.

## Plan

- **M1 (rates and plan price):**
  - Add per-model Claude rates to `spec/pricing.yaml` as a new revision. Cover Haiku, Sonnet and Opus 5.5: input, output, cache read, cache write (5-minute vs 1-hour, if they differ), and the Haiku >100k-input rate. Cite the source URL and date. Never put rates in Go.
  - Set `plan_prices.claude`, or pick the price by the live `plan_tier`, to Pro's monthly price as the owner confirms it.
  - Unit tests: a rate lookup per model, the >100k tier boundary at 100k−1, 100k and 100k+1, and an unknown model giving null rather than 0.
- **M2 (coverage and writer bugs):**
  - Find which sessions get per-turn rows (harnez agent jobs only? hooks?) and why host sessions like bd2185a4 have none. Then fix the gap, or document it as a limit #1302 must work around (reading transcripts instead).
  - Fix the duplicate quota-history writes.
  - Find why the Claude quota deltas are 0.
- **M3 (live canaries):** budgeted, read-only prompts, run with `harnez agent start --role advisor`.
  - **haiku:low:** about 5 short runs. Checks: rows written, `cost_micros` matching a hand calculation from the transcript's `usage` block, and a quota-delta row present.
  - **sonnet:low and opus:low:** at most 2 runs each, only after haiku passes. They check the higher-tier assumptions:
    - each model's rates are applied;
    - the quota % used per API dollar differs between models. `spec/agent.yaml` claims, unverified, that Haiku's subsidy is lower.
    - Weekly % moves in whole steps, so record the before and after reading, and read a "no change" result as "below resolution", not as zero.
  - Record each run's tokens, cost and quota before/after in this ticket.

## Verification

- Run the installed binary and read the rows with `sqlite3`.
- Hand-check one haiku and one opus canary against the JSONL transcript and the rates.
- `harnez usage --json` still reports `plan_tier` and the windows.

## Open facts (owner)

- The Pro monthly price, and whether it includes VAT or the price is in EUR.
- Whether to reject other machines' quota movement during canaries. Esprimo's last plan use is unknown.

## Decision

- 2026-11-26 (owner): the plan is **Claude Pro, priced in EUR**. Host default until the owner gives an invoice figure: **€18/month net**, without VAT, so it compares with net API prices. Two sources support it:
  - Anthropic's EU launch price, "€18 + VAT" (May 2024);
  - a third-party page from 2026-09-16 that gives €18 net and €21.42 with 19% German VAT ([SSD Nodes](https://www.ssdnodes.com/learn/lang/de/claude-pro-and-max-in-germany-what-you-pay)).

  The current price on claude.com is unverified. The owner overrides it with `agent.plan_prices` in `~/.harnez/config.yaml`.
- API rates are in USD. The spec must hold the plan currency and a USD→EUR rate with its source and date, so costs can be compared in one currency. The developer proposes the schema; keep existing `monthly_usd_micros` users working, or migrate them in the same commit.
- Sprint roles: developer `claude:haiku:med`. The host reviews; there is no reviewer agent, and opus is consulted only if really needed. The M3 canaries are run by the host, because the developer is a leaf worker and cannot call `harnez agent`. The developer supplies the canary prompts and the hand-check query.
- No long-context override in this sprint: that measurement belongs to #1302.

## Plan turn (dev-1301, 2026-11-26) and M1 pre-work

The developer confirmed the premises and corrected them:
- There are 4 zero-delta Claude rows (2 turns × 2 windows), not 2.
- All 112 Claude turn rows have null `cost_micros`.
- `opus` is stored as a bare model string, so the exact-key lookup in `internal/telemetry/economics.go:19` misses it.
- New bug: `claude.go:233` counts cache-creation as cached input, and `agent_run.go` subtracts it again, so **cache writes are priced nowhere**. `estimateCost` has no cache-write term, and a nil rate yields a partial total reported as complete.

Rates verified by the host with a direct fetch on 2026-11-26 from https://platform.claude.com/docs/en/about-claude/pricing. USD per MTok, in the order input / 5m write / 1h write / cache read / output:
- Opus 5.5: 4 / 5 / 8 / 0.20 / 20
- Sonnet 5.5: 2 / 2.50 / 4 / 0.10 / 10
- Haiku 5.5, prompts up to 100,000 tokens: 0.10 / 0.125 / 0.20 / 0.01 / 0.50
- Haiku 5.5, prompts over 100,000 tokens: 0.50 / 0.625 / 1 / 0.05 / 2.50

On the tier: the page says the prompt length "counts all of its input tokens, including cache reads and cache writes", and each request is priced on its own. That means per API call: the tier is decided per request, not per harnez turn total. Exactly 100,000 is the lower tier, by the page's "up to" / "over" wording; that is inferred, so test 99,999, 100,000 and 100,001. If the turn record cannot see per-request prompt sizes, say so and mark the cost as an estimate. Never price a multi-request turn on its summed input.

FX: ECB reference rate 1 EUR = 1.0935 USD on 2026-11-25 (https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/eurofxref-graph-usd.en.html). The plan price is €18/month net (see Decision).

Haiku 5.5 context: 1M window, with native auto-compact at about 967K (https://code.claude.com/docs/en/model-config). Only harnez's 80k threshold compacts it early, which matters for #1302.

M1 go, with these changes to the plan:
- The schema as proposed, with `source` and `fetched` on every rate entry and on the FX block.
- Keep DB values in USD and convert the plan price once at load.
- Use the 1h write rate only if transcripts tell the two apart (`cache_creation.ephemeral_1h_input_tokens`); otherwise price at 5m and record that.

M2 go:
- Coverage limit: only harnez agent turns write per-turn rows. Document it in the usage or telemetry docs as the source rule for #1302.
- Zero deltas: prove or refute the "forced fetch does not refresh Claude" hypothesis with a unit test, then fix it.
- Duplicates: find the root cause first, with a reproduction test, then dedupe.

## M1 and M2 delivered (host review, 2026-11-26)

- **M1 (rates and plan price), 5a73c918:** Claude rates per API call, priced on each call's own prompt, with the 5m and 1h cache-write TTLs priced separately. Aliases for `haiku`, `sonnet` and `opus`. A missing rate gives nil, never 0. The plan is Pro at €18, converted once at ECB 1.0935, giving 19,683,000 µUSD a month.
- **M2 (quota readings and history file), 8cb01e20 and 028b5da8:** the Claude reading is stored in SQLite even when the cache lock is busy, and duplicate quota-history lines are removed. The full q1 run passed: 0 `--- FAIL`, 81 packages ok.
- The host checked the spec rates against the verified table: all 15 values match.

### M3 canaries (host, installed branch build, 2026-11-26 10:46–10:47 UTC)

Prompt: "Reply with exactly OK…", about 20k prompt tokens, all in the lower tier. Costs in µUSD:

| run | model in row | in | cache read | cache write | out | cost | hand calc |
|---|---|---|---|---|---|---|---|
| haiku h1 | claude-haiku-5-5 | 2 | 11462 | 8762 | 4 | 1869 | 0.2+114.6+1752.4 (1h)+2 = 1869 ✓ |
| haiku h2 | claude-haiku-5-5 | 2 | 11422 | 8762 | 4 | 1868 | ✓ |
| haiku h3 | claude-haiku-5-5 | 2 | 20184 | 0 | 4 | 204 | 0.2+201.8+2 = 204 ✓ |
| sonnet | `sonnet` | 2 | 9449 | 9912 | 4 | 40637 | 4+945+39648 (1h)+40 = 40637 ✓ |
| opus | `opus` | 2 | 9333 | 10151 | 4 | 83163 | 8+1867+81208 (1h)+80 = 83163 ✓ |

- All cache writes in these runs used the 1h TTL.
- **No quota-delta rows were written for any of the 5 canaries.**
  - Their boundaries exist, but h1's before and after readings are the same cached snapshot (cache age 98.7 s, then 101.6 s).
  - h3's after reading got **HTTP 429** from the usage API.
  - The zero-delta and missing-row problem is therefore not fixed live.
  - Short turns are below the 1% resolution anyway, so the sonnet-vs-opus quota-per-dollar comparison cannot be measured with canaries. It moves to #1302, which uses windowed quota-history over many sessions.

## M3 pre-work (fixes from review)

1. **Price each call by its own model.**
   - The parser now also collects calls with a `Parent` (native subagents), but prices them at the session model's rates, so a Haiku subagent under an Opus session is priced as Opus.
   - Use the call's `message.model`.
   - Store the full model id (`claude-opus-5-5`), not the alias `opus`. A bare alias would silently get the wrong rates once a newer Opus ships.
   - Test: a mixed-model turn, and an unknown call model making the turn nil.
2. **Make the quota-delta outcome explicit.**
   - When the after reading is the same observation as the before reading, or the fetch failed (HTTP 429), write nothing silently? No: record why. Use whatever shape fits the existing tables, for example a delta row with an error or "unchanged-snapshot" source, or a documented skip reason.
   - Do not add retries that hit the usage API harder: the 429 is real.
   - Test: same-snapshot gives the documented outcome; 429 gives an error recorded, not a 0 delta.
3. **Fix the cumulative cache-write count.** The cumulative `turn_token_usage` row still counts cache writes in `cached_input_tokens` (your open item). Fix it so cumulative and delta rows agree.
4. **Leave the agy price alone.** Its USD price stays as it is: out of scope.
5. **Tests:** run only the touched packages (`go test ./internal/subagent ./internal/telemetry ./internal/usage ./cmd/harnez`). The full run before merge is owed by the host.
