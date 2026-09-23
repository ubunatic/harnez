# Plan quota cost per typical turn (2026-09-24)

Scope: local files only; estimate one typical turn on similarly priced subscriptions.

| Model | Typical-turn tokens | Points/turn | ×luna | Sample n | Confidence |
|---|---:|---:|---:|---:|---|
| gpt-6-luna | ? | ? | 1 (baseline) | >11 turns mentioned in reference; not recomputed | Low |
| gpt-6-astra | 18.7k new | ~2 points | 100 (known reference) | 1 | Low; integer quota rounding |
| Claude models | ? | ? | ? | Not measured | Insufficient |
| gemini-3.7-flash (agy) | ? | ? | ? | 1 agent session record | Very low |

## Findings

- The known Codex comparison is 2026-09-23 21:37–21:51 UTC: one 18.7k-new-token astra turn moved 5h usage 0→2%; 11 earlier luna turns left it at 0%. This supports a reference ratio near 100 only as a rounded, censored comparison, not a fitted rate.
- I did not calculate a median per-turn token count: the sampled rollout event parser did not match this machine's wrapped `payload` format; the opened recent rollout had no matching context/token events in that pass.
- The quota file has 24,291 rows; its latest inspected readings were codex 3%, Claude 41%, and agy 3% on their 5h windows. These are current levels, not attributable cost measurements.
- The five available harnez agent records include one agy session (`gemini-3.7-flash`), but cumulative cached tokens dominate and timestamps span a session, so they cannot yield a typical-turn estimate.
- Claude transcript fitting was not attempted; host Claude activity and model-attributed token sums need aligned interval parsing to separate model effects.
- Quota percentages are integer-rounded; small changes and zero changes cannot establish zero cost.
- Result: no defensible comparable per-model cost table can be inferred beyond the supplied astra/luna reference. Leave missing values unknown rather than extrapolating.

## Host follow-up (claude:opus, same day)

The agent's parser missed the format: Codex rollouts wrap events as
`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":…},"rate_limits":{"primary":{"used_percent":…,"resets_at":…}}}}`
(plan_type `plus`); `grep token_count` also matches prompt text. Per rollout (Sept 2026,
null rate_limits skipped, rollouts spanning a 5h reset excluded): new = input − cached +
output from `last_token_usage`, points = last − first primary `used_percent`.

| Model | Plan | Rollouts | pts / 100k new | × gpt-6-luna |
|---|---|---:|---:|---:|
| gpt-6-luna | ChatGPT Plus | 10 | 0.65 | 1 |
| gpt-5.6-luna | ChatGPT Plus | 213 | 1.87 | 3 |
| claude haiku / sonnet / opus (fit, step 2b) | Claude Pro | 884 intervals | 5.0 / 6.0 / 7.2–8.4 | 8 / 9 / 11–13 |
| gpt-5.6-terra | ChatGPT Plus | 27 | 10.5 | 16 |
| gpt-5.6-sol | ChatGPT Plus | 98 | 20.1 | 31 |
| gpt-6-astra | ChatGPT Plus | 15 | 40.6 | 62 |

Caveats: a rollout's delta includes concurrent Codex agents (overlap inflates busy
periods); integer `used_percent`; per token, not per turn (turn counts belong to EFF).
Cross-vendor comparison assumes equal-priced plans. Codex weekly hit 99% once in Sept.
Applied: astra/luna 62 is inside the 50–200 band, so multiples × 1.6 put astra at 100.
