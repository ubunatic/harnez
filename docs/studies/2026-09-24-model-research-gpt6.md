# GPT-6 dispatch table research

## Question
Do OpenAI’s published sources support the GPT-6 Codex table entries for codex:luna, codex:sol, and codex:astra?

## Method
Web research only on 2026-09-24. Reviewed official OpenAI API model catalog, model pages, reasoning and code-generation guidance, and dated changelog. Treated API model identifiers as evidence for model existence, not proof of internal Codex provider routing. Required model-specific benchmarks for language, TUI, and SQL skill claims.

## Findings
- All three names exist in the official GPT-6 API family: `gpt-6-luna`, `gpt-6-sol`, and `gpt-6-astra` ([model catalog](https://developers.openai.com/api/docs/models)).
- API pages show stable public IDs, not revision/version strings; separate immutable version numbers are not established. OpenAI release note dates Sol and Luna to Sep 22, 2026 ([changelog](https://developers.openai.com/api/docs/changelog)).
- The public provider is OpenAI API. Whether `codex:luna/sol/astra` internally routes to those endpoints is undocumented; no inference from API availability.
- Luna and Sol support `none`, `low`, `medium`, `high`, `xhigh`, and `max`; Astra supports `low`, `medium`, `high`, `xhigh`, and `max`. Thus effort is supported on each, including low/medium; `EFFORT yes` is consistent. “no” is not applicable to these rows ([Luna](https://developers.openai.com/api/docs/models/gpt-6-luna), [Sol](https://developers.openai.com/api/docs/models/gpt-6-sol), [Astra](https://developers.openai.com/api/docs/models/gpt-6-astra)).
- OpenAI calls Luna efficient for focused high-volume work and Sol a complex coding/agentic workflow model; Astra is positioned for the hardest end-to-end work ([model guidance](https://developers.openai.com/api/docs/guides/latest-model), [code generation](https://developers.openai.com/api/docs/guides/code-generation)). These support broad positioning, not comparative Go/TUI/SQL skill ratings or table-specific roles.
- No model-specific Go, terminal UI, or SQL evidence found in reviewed official sources. The Luna “weak on long terminal loops,” Sol “over-refactor,” Sol milestone reviewer, and Astra escalation-only claims require controlled workload measurements; none were found.
- “less prone to bending tests” is a test-following comparison; “terminal automation” needs success-rate/latency data; “interface/design changes” needs paired task results. No model-specific evidence found for these uses.

## Inconclusive
Public API documentation does not expose Codex’s provider alias mapping, internal model build/version, the table’s cost/effort-token units, or benchmark results for Go, TUI, SQL, refactoring, test adherence, terminal loops, or reviewer quality.

## Next time
Obtain authorized routing/config documentation and run paired, repeatable Codex evals for Go, TUI, SQL, terminal loops, refactoring, test adherence, and review; report latency and success criteria separately by exact route and effort.
