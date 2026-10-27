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

## 2. Findings (to verify; nothing below is confirmed yet)

- Model ID: unverified; `gpt-6.1-terra` by analogy with `gpt-6.1-sol`. Confirm from OpenAI's model page and from the
  installed Codex CLI's model list.
- Needed from the web, each with a source URL and the effort level it was measured at:
  context window, max output, price per MTok (input/output/cache), long-context price step if any (Luna and Sol 5.6
  have one above 272k), benchmarks for tool calling, research and coding, and how it sits between Luna and Sol.
- Old alias: 715 made old `terra` sessions fail clearly. Reusing `terra`/`terra:low` as aliases for the new model must
  not silently resume an old GPT-5.6 Terra session.
- Entry shape: follow `codex:sol` in `spec/agent.yaml` (aliases, provider, name, tier, roles, cost, cost_by_tier, eff,
  skills, use, guidance source). Cost values are owner-guided relative estimates (`codex:luna:low` = 1).

## 3. Implementation & Verification Plan

1. Research: a cheap web researcher collects the numbers above; mark vendor-marketing or blog claims unverified.
2. Owner decides alias, roles, cost estimate and one-line `use` guidance from the research.
3. Add the spec entry and update guides that list the model family (`docs/Models.md`, `docs/ModelResearch.md`,
   `docs/practices/ModelRoles.md` source plus root copy) and any test asserting the model table.
4. Verify: `harnez agent models` shows the three tiers; a live `harnez agent start --model terra:low` canary runs one
   turn and reports `gpt-6.1-terra` (or the confirmed ID); `make test-q1` passes; `make install`.
