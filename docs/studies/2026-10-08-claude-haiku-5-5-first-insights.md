# Claude Haiku 5.5: first insights

Date: 2026-10-08. Context: issue 755 (add Haiku 5.5 to the harnez model list). Haiku 5.5 was released 2026-10-07. All figures are from launch-week coverage, not from long-term use. Re-check before relying on them.

Rule for this study: do not compare Haiku 5.5 with Haiku 4.5 figures. Different generation, different numbers.

## Decision

`haiku` in harnez now means `claude-haiku-5-5` (`spec/agent.yaml`, `internal/claude/apply.go`). Roles: developer, reviewer, mechanical.

## Price

Pricing table (per million tokens), as shown on the web page the user provided. The page URL is not recorded here, so verify it against Anthropic's pricing page:

| Meter | Haiku 5.5, up to 100K | Haiku 5.5, over 100K | Haiku 4.5 (reference only) | Sonnet 5.5 (reference only) |
|---|---|---|---|---|
| Input | $0.10 | $0.50 | $1.00 | $2.00 |
| Output | $0.50 | $2.50 | $5.00 | $10.00 |
| Cache read | $0.01 | $0.05 | $0.10 | $0.10 |
| Cache write | $0.125 | $0.625 | $1.25 | $2.50 |

The 100K threshold changes the rate for the whole request, not just the tokens above it. Batch pricing is not in the table; sources disagree on the batch rate. A reseller lists $0.08 / $0.40, which is not the list price.

Check the list price on Anthropic's pricing page before any spec cost value is set.

## Capabilities

- Context window: 1M tokens input, 128K tokens max output.
- Anthropic reports no SWE-bench Verified score for this model.

## Leaderboard data (second screenshot, source page not recorded)

Score and per-task cost. The table's other columns are unlabelled and not used here.

| Model | Score | Cost per task |
|---|---|---|
| Opus 5.5 (Low) | 60.57 | $0.37 |
| Sonnet 5.5 (Medium) | 60.54 | $0.20 |
| GPT-6 Sol (High) | 59.92 | $0.31 |
| GPT-6 Luna (Max) | 59.07 | $0.03 |
| Haiku 5.5 (High) | 58.63 | $0.05 |
| Haiku 5.5 (Medium) | 58.20 | $0.03 |
| GPT-6 Luna (Xhigh) | 56.40 | $0.03 |
| GPT-6 Sol (Medium) | 56.32 | $0.21 |

Haiku medium scores above Sol medium on this board at about one seventh of the cost. Haiku uses the Claude Code harness and Sol uses Codex CLI, so this is not a like-for-like comparison.

The spec's cost for Haiku was set from these figures: medium 4, high 7 (Sol medium is 29 on the same scale).

## Benchmarks (vendor or third-party, mostly at max effort)

| Benchmark | Haiku 5.5 | Notes |
|---|---|---|
| SWE-Bench Pro | 64.8% | Sonnet 5.5: 81.3% |
| SWE-bench Multilingual | 83.7% | Sonnet 5.5: 90.3% |
| OSWorld 2.1 (offline subset) | 72.4% | GPT-6 Luna: 48.9% |
| Terminal-Bench 4.0 | 39.2% at max effort | 12.7% at low effort, 31.5% at very high. Single third-party write-up. GPT-6 Luna: 16.4% |
| GDPval-AA v2.1 | 1,620 Elo | GPT-6 Luna: 1,437 |
| Artificial Analysis Intelligence Index | 43.4 | GPT-6 Luna: 38.1. Haiku at default (medium) effort: 34.5 |

Effort matters a lot. Terminal-Bench 4.0 ranges from 12.7% (low) to 39.2% (max), and cost rises at each step.

## Comparison with the other agent models

| Model | Terminal-Bench 4.0 | Other data | Cost signal |
|---|---|---|---|
| Haiku 5.5 | 39.2% (max) | OSWorld 72.4%, GDPval 1,620 | about $0.21 per finished task (Artificial Analysis run) |
| GPT-6.1 Sol | 58.2% ± 3.1 pp (max) | 45 Elo below Haiku on one knowledge benchmark | not reported in the source |
| GPT-6 Luna | 16.4% | GDPval 1,437 | about $0.068 per finished task |
| Gemini 3.7 Flash | no overlap found | AutomationBench 30.4%, Code Arena 1588 Elo (Google launch charts) | not ranked |

Gaps to keep in mind:
- The runs used different agent setups, so they are not a like-for-like test.
- `sol:med` and `luna:high` were not reported separately. Every number above is from a max-effort or vendor run.
- The Gemini figures come from Google's own charts and do not overlap with the other three models.

## Early user reports (launch week, preliminary)

- Praised for speed and cost. Asana reports over 30% lower task-completion latency (vendor-cited). A HubSpot engineer's quote on Anthropic's page says it is fast enough for their agent product.
- Accuracy is mixed. One tester saw a wrong answer on a simple logic question.
- A small informal comparison against DeepSeek V4.1 Flash found Haiku faster and cheaper at low effort, but weaker on judged results. Four briefs, one run each. Not a ranking.
- One small side-by-side test: Haiku passed 60 of 80 outputs (75%), Luna 51 of 80 (64%). The author warns against treating it as a general ranking.
- Cost can creep up: agent sessions that cross the 100K-token threshold, or leave the effort setting high, cost more per request.
- Haiku is more verbose than Luna on the same run: about 440 million output tokens against 140 million.

## Open questions

- Reliability on reasoning-heavy coding in a long agent loop. No independent data yet.
- Whether Haiku at max effort is worth the cost compared with medium effort, on harnez's own tasks.
- The `cost` score in `spec/agent.yaml` is still the Haiku 4.5-era value of 12. It needs a value based on the confirmed price.

## Next steps

1. Confirm list prices on Anthropic's pricing page and update the spec cost value.
2. Run a small harnez bench (`harnez bench run`) of `claude:haiku:low` against `codex:sol:med` and `codex:luna:high` on the same tasks, so the comparison comes from our own runs.
3. Revisit this study after two weeks of use, and add independent leaderboard results when they appear.

## Sources

- [VentureBeat: Haiku 5.5 launch and pricing](https://venturebeat.com/technology/anthropic-launches-claude-haiku-5-5-with-90-api-price-reduction-matching-gpt-6-luna)
- [OpenRouter: Haiku 5.5 pricing and context](https://openrouter.ai/anthropic/claude-haiku-5.5)
- [LiteLLM: Haiku 5.5 day-0 support](https://docs.litellm.ai/blog/claude-haiku-5-5)
- [Morph: Claude benchmarks](https://www.morphllm.com/claude-benchmarks)
- [Technology Org: Haiku 5.5 price and benchmarks](https://www.technology.org/2026/10/08/claude-haiku-5-5-anthropic-price-benchmarks/)
- [ComputingForgeeks: Haiku 5.5 benchmarks](https://computingforgeeks.com/claude-haiku-5-5-released-features-benchmarks/)
- [Artificial Analysis: Haiku 5.5](https://artificialanalysis.ai/articles/claude-haiku-5-5)
- [Anthropic: Introducing Haiku 5.5](https://www.anthropic.com/claude-haiku-5-5)
- [Handy AI: Haiku 5.5 model drop](https://handyai.substack.com/p/model-drop-claude-haiku-55)
- [Decrypt: Haiku 5.5 launch](https://decrypt.co/380351/anthropic-launches-haiku-5-5-cheapest-fastest-claude-model)
- [Kingy AI: GPT-Sol, Luna vs Haiku 5.5](https://kingy.ai/blog/gpt-sol-luna-vs-claude-haiku-5-5/)
- [OrcaRouter: Haiku 5.5 vs GPT-5.6 Luna](https://www.orcarouter.ai/blog/claude-haiku-5-5-vs-gpt-5-6-luna)
