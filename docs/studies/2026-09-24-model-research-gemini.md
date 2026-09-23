# Gemini Model Row Audit (2026-09-24)

## Question
Are the `agy:flash37` and `agy:flash38` spec rows correct on exact ID/version, AGY route, effort, Go/TUI/SQL skills, roles, and use?

## Method
Web research only; no repository inspection. Checked Google model documentation, Google Antigravity CLI documentation, DeepMind model cards, and latency provider data. Treated each skill and prose use phrase as a separate empirical claim; absence of model/task-specific evaluation is inconclusive.

## Findings
- `gemini-3.7-flash` and `gemini-3.8-flash` are exact current Google model IDs, corresponding to the named generations; Google lists both as stable. [3.7 model page](https://ai.google.dev/gemini-api/docs/models/gemini-3.7-flash) [current models](https://ai.google.dev/gemini-api/docs/models)
- AGY publishes effort-qualified model slugs for both families, including low and medium; its CLI documents the low/medium/high effort control. This confirms AGY model selection but does not establish the underlying provider backend route for each request. [AGY CLI](https://www.antigravity.google/docs/cli/headless/)
- Google documents thinking controls at low, medium, and high for Gemini 3.7 Flash; the AGY CLI list independently supports low/medium qualified options for 3.7 and 3.8. Thus “EFFORT yes” is supported; “no” would mean unsupported only if the spec defines it that way, not a finding inferred here.
- DeepMind reports coding/agent results for 3.8 (including Terminal-Bench 2.1 and DeepSWE), but not Go-specific, terminal-interface-specific, or SQL-specific ability evidence. No model-specific evidence found for either generation on those three claims. [3.8 model card](https://deepmind.google/models/model-cards/gemini-3-8-flash/) [3.7 model card](https://deepmind.google/models/model-cards/gemini-3-7-flash/)
- Broad benchmark evidence makes coding/review and agentic tool use plausible, but does not verify the spec's bounded-workflow recommendations, relative role ordering, or “cross-vendor reviewer” characterization for AGY-routed Gemini. The “slow first token” claim requires measured TTFT on the AGY route; provider measurements exist, but are not AGY-specific. [provider latency dashboard](https://artificialanalysis.ai/models/gemini-3-8-flash/providers)

## Inconclusive
No primary, reproducible AGY-route Go/TUI/SQL task benchmarks, role comparisons, or AGY-specific first-token measurements found. “Cross-vendor” describes a workflow relationship, not model capability, and needs local workflow evidence.

## Next time
Capture dated `agy models` output and route metadata; run fixed Go, terminal UI, SQL, review/advisor/developer tasks with scored checks and AGY TTFT instrumentation before asserting comparative skills or use recommendations.
