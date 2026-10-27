---
kind: topic
title: GPT-6.1 Sol, GPT-6.1 Terra and GPT-6 Luna comparison
---

# GPT-6.1 Sol, GPT-6.1 Terra and GPT-6 Luna comparison

## Context

On 2026-10-27 we compared coding capability, speed and cost to guide Harnez agent selection and [Terra onboarding](../../issues/974-onboard-gpt-6-1-terra-codex-into-the-model-list-with-sourced-numbers.md). This is a dated research snapshot, not a Harnez benchmark or an implementation decision.

## Outcome

### Cost and limits

OpenAI's [API pricing](https://developers.openai.com/api/docs/pricing) gives these standard rates per million tokens for short-context requests:

| Model | Fresh input | Cached input | Output |
| --- | ---: | ---: | ---: |
| GPT-6.1 Sol | $2.00 | $0.10 | $10.00 |
| GPT-6.1 Terra | $1.00 | $0.05 | $5.00 |
| GPT-6 Luna | $0.10 | $0.01 | $0.50 |

These are API rates, not Codex subscription charges or task costs. The [Terra model page](https://developers.openai.com/api/docs/models/gpt-6.1-terra) lists 1,050,000 context tokens, 128,000 maximum output tokens, and `none`, `low`, `medium`, `high`, `xhigh`, and `max` reasoning effort. Its standard 30-minute cache write costs $1.25 per million tokens. Prompts above 272K input tokens incur 2x input and cache rates and 1.5x output for the whole request. API model documentation does not establish installed Codex or Harnez availability.

### Coding and speed

Independent research previously cited in this session placed Sol ahead of Terra, and Terra ahead of Luna, on a composite coding-agent index; those are index points, not SWE-bench percentages. No directly comparable three-model SWE-bench result was established. The models' [official API pages](https://developers.openai.com/api/docs/models/gpt-6.1-terra) establish capability and price, not a comparable three-model coding score; preserve that distinction until the underlying benchmarks and effort settings are independently verified.

Reported output throughput and end-to-end task time measure different things. No verified independent Terra throughput was established here. A comparison using different reasoning effort, workload or provider cannot establish how quickly a Harnez sprint will finish. As a working hypothesis, use Luna for bounded low-cost tasks, evaluate Terra for ordinary coding, and reserve Sol for harder judgment; validate on local workloads before changing role guidance.

The session's `luna:med` agent returned 15 ranked candidate issue numbers from a supplied list of 46. This shows the format was possible, not that its ranking was correct. Harnez inferred a reviewer role from the prompt; the result does not establish plain-agent behavior.

## What Went Wrong

- A prior comparison mixed independently measured token throughput with a vendor's task-completion speed claim. They are different metrics and workloads; the comparison above labels the distinction and leaves Terra throughput unverified.
- The initial Terra ticket treated its ID, limits and prices as unverified. The official model page establishes API facts; Codex availability remains subject to the live canary in issue 974.
- No comparable three-model SWE-bench result was found. Avoid treating composite index scores or different evaluation harnesses as interchangeable SWE-bench percentages.

## Learnings

- Include reasoning effort, benchmark version, source, and measurement conditions alongside any model score or speed figure.
- Evaluate total task cost and success rate, including retries, rather than deriving both from token prices or output speed.
- Recheck live model availability and preserve old session identity before reusing a model alias.

## Follow-Up Tickets

- [974](../../issues/974-onboard-gpt-6-1-terra-codex-into-the-model-list-with-sourced-numbers.md): finish source verification, decide alias/roles/cost guidance, onboard Terra, and run the live Codex canary.
- [978](../../issues/978-groom-open-issues-into-backlog-with-a-ranked-candidate-list.md): explore a reviewable issue-grooming flow. Local Neus ranking has its own issue in the sibling repository.

No new ticket is needed for the benchmark and canary gaps: issue 974 already covers them. No model configuration, production role policy, or independent benchmark was changed in this study.
