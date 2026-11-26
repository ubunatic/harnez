---
kind: sprint
date: 2026-11-26
ticket: 1301
branch: sprint/1301
models:
  searcher: skipped # ticket written against live code by the host the same session (from the 1300 assessment)
  developer: claude:haiku:med
  reviewer: host only # owner: "you review, opus as consult only if really needed"
---

# Two-Agent Lean Sprint: #1301 — Claude cost inputs for usage analytics

## Context

The #1300 assessment showed that `harnez usage` could not answer the owner's Claude cost questions: API cost vs. Pro plan cost, the subsidy factor, and quota per session. #1301 closes the input gaps. #1302 builds the report on top.

## Milestones

| Milestone | Commits | Result |
|---|---|---|
| M1 per-call pricing, Pro EUR price | 5a73c918 | Claude rates in `spec/pricing.yaml`, including the Haiku >100k tier; 5m/1h write split per message id; Pro €18 net with ECB FX in `spec/agent.yaml` |
| M2 quota capture | 8cb01e20, 028b5da8 | Snapshot stored under a busy lock; quota history deduped |
| M3 review fixes | b8fb16c3 | Price each call by its own model; `turn_quota_pair_outcomes`; cumulative row excludes cache writes |
| M4 canary fixes | 027a5f5a | Pairing allows reset-time jitter, and reused snapshots are classified by `captured_at − cache_age`; tolerances in `spec/usage.yaml`; rows store the reported model id |

## Verification

- Live canaries: 3× haiku:low, 1× sonnet:low, 1× opus:low (M2), and 1× haiku:low after each of M3 and M4. Every stored cost matched a hand calculation from the stored tokens and the verified rates. Example: M4 haiku, 2 + 11422 read + 8764 1h write + 4 out = 1869 µUSD.
- The M4 canary is classified `unchanged_snapshot`: both snapshot times work out to 10:58:48.99. A live `paired` outcome needs a fresh usage fetch inside one turn. The unit tests cover that case with the live jitter values, but no live turn has shown it yet.
- Developer touched-package runs after M3 and M4 had 0 failures. The host full q1 runs are recorded below.

## Observations

- **Canaries found what green tests hid, twice.** M3's `unchanged_snapshot` test used a fixture that the live write path never produces, because every boundary writes its own observation. The real reason no row paired was sub-second jitter in `reset_at` (12:10:00.065 vs 12:09:59.889), not the cached snapshot the host first suspected. Both surfaced only by reading the live DB rows of a $0.002 canary.
- **The host's own first diagnosis was partly wrong.** M2's review blamed the missing deltas on the cached snapshot and the 429. Both are real, but neither was the blocker. The rule "one measurement is a hypothesis" held here.
- **Short turns cannot measure quota per dollar.** One "reply OK" turn stays below the 1% step of the weekly limit. Other Claude sessions on the same plan (../splitty, ../taia) also move the quota during a turn. So the sonnet vs. opus quota-per-dollar comparison moved to #1302, as totals over time windows.
- **The haiku:med developer followed pre-work well.** It also flagged its own deviations: one existing count assertion changed (32 → 33 spec keys), red tests not observed separately, and a behaviour change (no 0-delta row for a reused snapshot).
- **Skipped the agent reviewer.** The owner asked for host review and Opus only when needed. The skill's default reviewer resolves to Opus, so the host reviewed every diff and used live canaries instead.

## Follow-ups

- #1302: range and session cost report, time of day, factor graph, and the long Haiku session over 100k context (raise the harnez Haiku compact threshold above ~170k first).
- A live `paired` outcome is still to be observed. It should appear in #1302's longer sessions.
- Sessions resumed from before M3 keep the old cumulative cached semantics (documented, no migration).
