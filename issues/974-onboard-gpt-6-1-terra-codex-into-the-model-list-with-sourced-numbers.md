# 974 — Onboard GPT-6.1 Terra (Codex) into the model list with sourced numbers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 715 (removed the older GPT-5.6 Terra), 504 (onboarded GPT-6 Sol and Luna), 942 (model facts need sources), 972 (verify before claiming), 973 (Haiku tiers; Luna/Haiku comparison)

---

/goal `harnez agent models` lists GPT-6.1 Terra with low/med/high tiers, every number in its `spec/agent.yaml`
entry backed by a source URL or marked unverified, and a live Codex canary that resolves and runs it; or stop and
report when blocked on an owner decision (alias, cost, roles) or the model is not available in the installed Codex.

## 1. Problem & Motivation

Owner, 2026-10-27: a new GPT-6.1 Terra model is out and should be onboarded. Harnez currently lists Luna (`gpt-6-luna`),
Sol (`gpt-6.1-sol`) and Astra (`gpt-6-astra`), but no Terra. The older GPT-5.6 Terra was removed in 715 as an
older-generation model; GPT-6.1 Terra is a new model, so 715's decision does not apply to it.

## 2. Findings (research snapshot 2026-10-27)

- [OpenAI's model page](https://developers.openai.com/api/docs/models/gpt-6.1-terra) confirms the API ID `gpt-6.1-terra`, 1,050,000-token context, 128,000 maximum output tokens, and reasoning efforts none/low/medium/high/xhigh/max. Installed Codex availability: the owner uses Terra in Codex
  (2026-10-27); the harnez canary in section 3 still confirms the ID it resolves to.
- The same page gives standard API prices per million tokens of $1 input, $0.05 cached input, $5 output, and $1.25 for 30-minute cache writes. Above 272K input tokens, the full request is priced at 2x input/cache and 1.5x output. These are API rates, not measured Harnez or Codex task costs.
- [Comparison study](../docs/studies/2026-10-27-codex-sol-terra-luna-comparison.md): Terra sits between Sol and Luna in the independent coding-agent comparison consulted. Comparable three-model SWE-bench scores and independent Terra throughput remain unverified. Source benchmark, effort, workload, and conditions before using them in guidance; tool-calling and research benchmarks still need review.
- Old alias: 715 made old `terra` sessions fail clearly. Reusing `terra`/`terra:low` as aliases for the new model must
  not silently resume an old GPT-5.6 Terra session.
- Entry shape: follow `codex:sol` in `spec/agent.yaml` (aliases, provider, name, tier, roles, cost, cost_by_tier, eff,
  skills, use, guidance source). Cost values are owner-guided relative estimates (`codex:luna:low` = 1).

## 2b. Searcher findings (2026-10-27, luna:low, search-974)

- Entry shape: `codex:sol` (`spec/agent.yaml:97`): aliases `sol, sol:low`, cost 29, cost_by_tier med 47 / high 71,
  roles developer/advisor/reviewer. `codex:luna` (`:84`): cost 1, med 4 / high 6. Costs are owner estimates, not
  API price ratios; `internal/subagent/agentspec_test.go:82` asserts them.
- Old alias: `internal/subagent/driver_test.go:57` asserts `terra`, `terra:low`, `codex:terra` resolve as unknown
  (715). Resume passes the resolved model to Codex (`-m`), so reusing `terra` would let a session recorded as
  `terra` resume on GPT-6.1 Terra; that needs explicit handling, not just new aliases.
- Write set: `docs/Models.md` (cost table :44, API price table :75, Codex family comparison ~:143),
  `docs/ModelResearch.md` (:65, :99), `docs/practices/ModelRoles.md` (:70) plus root copy `docs/ModelRoles.md`,
  `internal/subagent/testdata/known-model-specs.golden` (:24), `agentspec_test.go` (:42), `driver_test.go` (:57).
- Installed Codex 0.164.0: `~/.codex/models_cache.json:272` lists `"slug": "gpt-6.1-terra"`.
- OpenAI model page still matches section 2. Benchmarks: OpenAI claims Terminal-Bench 4.0 Terra 50.3% at max
  ([announcement](https://openai.com/index/introducing-gpt-6-1-terra/), vendor claim). Secondary report of
  Artificial Analysis: Terminal-Bench 4.0 Terra 56%, Sol 56%, Luna 31%; Terra 91 tokens/s vs Sol 69, effort not
  stated ([MultiAI](https://multiai.co/blog/gpt-6-1-terra), secondary, unverified). No sourced independent
  SWE-bench or tool-calling result.
- List-price ratio for a 100k-input / 20k-output, half-cached turn: Luna 1x, Terra 10x, Sol 20x.

## Decision (owner, 2026-10-27)

1. Aliases reuse the old names: `terra`, `terra:low`, `terra:med`, `terra:high`, `codex:terra` resolve to
   `gpt-6.1-terra`. Guard: a session recorded with the old GPT-5.6 Terra model must not resume on the new model; it
   fails with a clear message naming the old model (715's intent kept for old sessions).
2. Roles: developer, advisor, reviewer.
3. Cost: owner reading of list prices, Terra is 0.5x Sol and 10x Luna (5x on cached input). The two anchors
   disagree on the current scale (Sol low is 29, not 20x Luna), so the host picked 0.5x Sol per tier:
   cost 15, cost_by_tier med 24 / high 36. Reversible.
4. Use line: nearly as good as Sol at half its price. Owner-quoted Terminal-Bench scores Sol 54, Terra 48, Luna 31
   (owner figures, source not recorded; section 2b's sourced figures differ). Guidance text:
   "Near-Sol Codex developer and reviewer at half Sol's price; use Sol for the hardest judgment."
   Docs cite the sourced numbers from 2b and mark the owner figures as unverified.

## Milestones

- **M1 (Terra entry and old-session guard):** spec entry after `codex:sol` per Decision; flip
  `driver_test.go:57` so the aliases resolve to `gpt-6.1-terra`; add a test that a session recorded with the old
  GPT-5.6 Terra model is refused on resume with a message naming it; update golden, `agentspec_test.go` cost and
  guidance assertions, and the docs in 2b's write set (ModelRoles source and root copy). Verify per section 3,
  live canary `harnez agent start --model terra:low` reporting `gpt-6.1-terra`. One commit naming issue 974 M1.

## 3. Implementation & Verification Plan

1. Research: fill the remaining benchmark gaps above and verify sources and effort settings; distinguish vendor claims from independently measured results.
2. Owner decides alias, roles, cost estimate and one-line `use` guidance from the research.
3. Add the spec entry and update guides that list the model family (`docs/Models.md`, `docs/ModelResearch.md`,
   `docs/practices/ModelRoles.md` source plus root copy) and any test asserting the model table.
4. Verify: `harnez agent models` shows the three tiers; a live `harnez agent start --model terra:low` canary runs one
   turn and reports `gpt-6.1-terra` (or the confirmed ID); `make test-q1` passes; `make install`.
