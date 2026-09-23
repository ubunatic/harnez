# Model price and quota research — 2026-09-24

Question: Compare every model key in the supplied dispatch spec on official token pricing, subscription quota semantics, a fixed 100k input (50% cached)/20k output workload, and tokens used per successful coding task.

Method: Web research only; official provider pricing, model and help pages searched on 2026-09-24. Workload uses 50k uncached input + 50k cached input + 20k output at standard API prices. Ratio divides by GPT-6 Luna's $0.0155. Reasoning output tokens included where provider says so. No repo/local inspection.

Findings:
- OpenAI API list prices in the output column below are input/cached input/output. GPT-6 Luna workload costs $0.0155; Astra costs $1.55, exactly 100×. GPT-6 Sol is $0.31 (20×). Source: https://developers.openai.com/api/docs/pricing (accessed 2026-09-24).
- Gemini 3.7 Flash list is $0.75/$0.075/$3.75 per million through 2026-12-31, then $1.50/$0.15/$7.50; workload now $0.11625 = 7.5× Luna. Source: https://ai.google.dev/gemini-api/docs/pricing (accessed 2026-09-24).
- Gemini 3.8 Flash official announcement states introductory input/output $0.75/$3.75 through 2026-12-31 and standard $1.50/$7.50 after; cached rate taken as same Gemini 3 Flash pricing table ($0.075 now). Workload ratio 7.5×. Source: https://ai.google.dev/gemini-api/docs/latest-model (accessed 2026-09-24).
- Anthropic Opus 4.6 list $5/$0.50/$25; Sonnet 4.6 $3/$0.30/$15; Haiku 4.5 $1/$0.10/$5. Workload ratios are respectively 45.16×, 30×, 10×. Pricing schedule: https://www-cdn.anthropic.com/files/4zrzovbb/website/3684c2faafb97418665782cea0001f439f74b1d2.pdf (accessed 2026-09-24); Sonnet confirmation: https://www.anthropic.com/news/claude-sonnet-4-6.
- Anthropic subscription quotas are variable usage limits, not a public fixed token allowance for each model; usage window/reset limits depend on plan and product. No conversion to API dollars/tokens was inferred. Source: https://support.anthropic.com/en/articles/8324991-about-claude-pro-usage.
- OpenAI Codex subscription use shares plan quota across Work/Codex and depends on model/task/settings; five-hour and weekly limits apply, with optional paid resets on eligible personal accounts. No token conversion inferred. Source: https://help.openai.com/en/articles/20001516-managing-usage-with-gpt-6-astra-in-work-and-codex.
- Google API free tier/paid tier are documented; consumer AI subscriptions do not create a defensible fixed model-token conversion. Gemini 3.7 details: https://ai.google.dev/gemini-api/docs/pricing.
- EFF is ? for every model. Search surfaced benchmark success rates and prices but no independently comparable report of total tokens including retries per successful task, for a named harness/suite and these exact versions. Leaderboard score and speed were excluded.

Inconclusive: Exact subscription dollar prices/quota reset schedules vary by country, plan, product and account and do not translate into comparable token amounts. Gemini 3.8 cache price and Claude product quota details warrant confirmation from their live plan pages before production use. Gemini 3.8 is named by the spec as a model, but the cited official announcement supplies its rollout price, not a stable full model pricing entry.

Next time: Repeat the API price calculation from archived official rate cards; obtain actual per-run token logs (retries included) using one fixed coding-task harness and suite before assigning any EFF grade.
