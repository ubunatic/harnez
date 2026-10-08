# 755 — Add Claude Haiku 5.5 to the harnez model list

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `spec/agent.yaml`, `spec/pricing.yaml` (planned by 445), `harnez agent models`

---

> `/goal`: Add Claude Haiku 5.5 (`claude-haiku-5-5`) as a selectable harnez agent model with a verified cost tier and role, or stop and report when blocked on a user decision (for example, a price figure that cannot be confirmed) or a denied permission.

## 1. Problem & Motivation

harnez lists Claude Haiku only as `claude:haiku`, which points at the Claude CLI alias `haiku`. Claude Haiku 5.5 (`claude-haiku-5-5`) is now available, but harnez cannot select it by name, so it does not appear in `harnez agent models`. Users who want the cheapest Claude tier for mechanical work cannot pick the 5.5 generation.

## 2. Technical Specification / Findings

- Current entry: `spec/agent.yaml` `claude:haiku` with `name: haiku`, `tier: low`, `cost: 12`, role `mechanical`. It is the CLI alias, so it does not pin 5.5.
- Sibling 5.5 entries (`agy:opus`, `agy:sonnet`) use the full model ID (`claude-opus-5-5`, `claude-sonnet-5-5`). Haiku 5.5 should follow that pattern.
- `cost` is a relative score, not dollars. Haiku 4.5 figures must not be used as a proxy for 5.5 (different generation).
- Price (Haiku 5.5, launch coverage 2026-10-07/08; verify on Anthropic's page): $0.10 input, $0.50 output, $0.01 cache read, $0.125 cache write per million tokens for prompts up to 100K. Above 100K, the whole request bills at $0.50 input and $2.50 output. Batch is discounted; one source says $0.05 / $0.25, others say 50% off. A reseller lists $0.08 / $0.40, which is not the list price. Sources: [VentureBeat](https://venturebeat.com/technology/anthropic-launches-claude-haiku-5-5-with-90-api-price-reduction-matching-gpt-6-luna), [OpenRouter](https://openrouter.ai/anthropic/claude-haiku-5.5), [LiteLLM](https://docs.litellm.ai/blog/claude-haiku-5-5).
- Context: 1M tokens input, 128K max output. Source: [OpenRouter](https://openrouter.ai/anthropic/claude-haiku-5.5).
- Benchmarks (no SWE-bench Verified published by Anthropic):
  - SWE-Bench Pro 64.8% (Sonnet 5.5: 81.3%); SWE-bench Multilingual 83.7% (Sonnet 5.5: 90.3%). Per the system card as reported by [Morph](https://www.morphllm.com/claude-benchmarks) and [Technology Org](https://www.technology.org/2026/10/08/claude-haiku-5-5-anthropic-price-benchmarks/).
  - OSWorld 2.1 72.4%. Terminal-Bench 4.0 39.2% at max effort, rising with effort from 12.7% (low). Single third-party write-up: [ComputingForgeeks](https://computingforgeeks.com/claude-haiku-5-5-released-features-benchmarks/).
  - Artificial Analysis Intelligence Index 43: [Artificial Analysis](https://artificialanalysis.ai/articles/claude-haiku-5-5).
  - Independent SWE-bench and Terminal-Bench leaderboards had no 5.5 entries at the time of the search.
- User reports (launch week, preliminary): praised for speed and cost. Asana reports over 30% latency drop on task completions (vendor-cited). Accuracy is mixed: one tester saw a wrong answer on a simple logic question, and a small informal comparison found it weaker than DeepSeek V4.1 Flash on judged results (four briefs, one run each). Sources: [Anthropic launch](https://www.anthropic.com/claude-haiku-5-5), [Handy AI](https://handyai.substack.com/p/model-drop-claude-haiku-55), [Decrypt](https://decrypt.co/380351/anthropic-launches-haiku-5-5-cheapest-fastest-claude-model).
- Still missing: a reliability read on reasoning-heavy coding work in an agent loop. Check Reddit and developer-blog threads for newer hands-on reports.
- Open question: keep the existing `claude:haiku` alias and add a new entry, or repoint it to `claude-haiku-5-5`. Repointing changes what existing users get from `haiku`.
- Related: #445 plans rate cards for `haiku` (latest); the 5.5 price should feed that table once confirmed.

## 3. Implementation & Verification Plan

1. Confirm Haiku 5.5 pricing and capabilities on Anthropic's official pages; record the source here.
2. Add a spec entry in `spec/agent.yaml` for `claude-haiku-5-5` following the `agy:opus` pattern, with aliases and a cost tier chosen from the confirmed data. Use the user's decision on whether `haiku` repoints.
3. Run `make install`, then `harnez agent models` and confirm the new row appears with the right model ID.
4. Run the spec tests and any model-list tests, and confirm no Go code duplicates the spec values.
