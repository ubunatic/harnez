# Model Assessment

Operational role guidance lives in `spec/agent.yaml` (`roles`, `use`, `use_med` per model)
and prints with `harnez agent models`. This doc keeps the evidence behind it. Role
practice: `docs/practices/ModelRoles.md`; measured sprint eval: `ModelAdvisoryEval.md`.
To refresh it, follow [ModelResearch.md](ModelResearch.md).

## 2026-09-24 research refresh snapshot

First full run of [ModelResearch.md](ModelResearch.md): 5 web researchers (family + price)
and one local quota analyst, all `codex:luna:med`, in parallel, 52s–3m34s, ~30–54k new
tokens each. Table before: [data/models-001.txt](data/models-001.txt); after:
[data/models-002.txt](data/models-002.txt). Studies:
[gpt6](studies/2026-09-24-model-research-gpt6.md),
[gpt56](studies/2026-09-24-model-research-gpt56.md),
[claude](studies/2026-09-24-model-research-claude.md),
[gemini](studies/2026-09-24-model-research-gemini.md),
[price](studies/2026-09-24-model-research-price.md),
[quota](studies/2026-09-24-model-research-quota.md).

COST (workload 100k in / 20k out / 50% cached, list price, × luna):

| Model | List price in/cached/out USD per 1M | × luna | COST proposed (not kept) |
|---|---|---|---|
| codex:luna | 0.10 / 0.01 / 0.50 | 1 | 1 |
| agy:flash37, flash38 | 0.75 / 0.075 / 3.75 (flash38 introductory to 2026-12-31) | 7.5 | 4 → 8 |
| claude:haiku | 1 / 0.10 / 5 (low confidence) | 10 | 4 → 10 |
| codex:sol | 2 / 0.20 / 10 | 20 | 20 |
| codex:terra | 2 / 0.20 / 12 | 22.6 | 16 → 22 |
| claude:sonnet, agy:sonnet | 3 / 0.30 / 15 | 30 | 16 → 30 |
| claude:opus, agy:opus | 5 / 0.50 / 25 | 45 | 32 → 60 (user-stated opus ≈ 2× sonnet wins) |
| codex:astra | 10 / 1 / 50 | 100 | 100 (list ratio lands exactly on the anchor) |

Findings:

- **No cell except COST changed.** Every SKILLS, ROLES and USE claim came back `?`: no
  model-specific public evidence for Go, TUI or SQL exists. Per the reconcile rule the
  cells stay; only [ModelTrials.md](ModelTrials.md) canaries can settle them.
- **Claude effort canary (2026-09-24):** the installed CLI accepts `--effort low` in
  `-p` mode, and its help lists low/medium/high/xhigh/max. `harnez agent` passes the
  explicit model tier for Claude turns; `:med` maps to `medium`, and the default no-tier
  invocation remains unchanged.
- **Claude aliases move:** `sonnet`/`opus`/`haiku` resolve to the newest model (Sonnet 5,
  Opus 5.x, Haiku 4.5 today); the web evidence is partly for 4.6. Transcripts show
  `claude-opus-5`, `claude-opus-5-5`, `claude-sonnet-5`, `claude-haiku-4-5`.
- **EFF stays `?` everywhere:** no harness publishes tokens per successful task per model.
- **Local quota (Claude only, rough least-squares fit, 884 intervals):** 5h points per 100k
  new tokens: haiku 5.0, sonnet-5 6.0, opus-5-5 7.2, opus-5 8.4. Opus costs ~1.2–1.4×
  sonnet per token on the subscription, less than the list ratio; haiku is barely cheaper
  than sonnet, likely because cached tokens dominate. Codex and agy attribution was not
  defensible (overlapping account-wide readings).
- **COST reverted (same day):** list prices mislead across vendors (terra > sol, opus 60 vs astra 100 contradict measured quota); COST is now plan quota per turn, see ModelResearch.md step 3.
- **COST from plan quota (applied):** 5h points per 100k new tokens on ChatGPT Plus /
  Claude Pro, × luna, stretched × 1.6 so the measured astra ratio (62) lands on 100:
  luna 1, haiku 12, sonnet 15, opus 20, terra 26, sol 50 (gpt-5.6-sol data, gpt-6-sol
  unmeasured), astra 100. agy (Google Pro) unmeasured, kept at 4/16/32. Opus is only
  ~1.3× sonnet per token (user's 2× estimate likely includes turn counts, i.e. EFF).
  Details: [plan-quota study](studies/2026-09-24-model-research-plan-quota.md).
- **Process:** all six agents wrote their studies (run from a scratch dir, writing into the
  repo worked). The quota agent first read "read-only" as forbidding the study file; say
  "read-only except the study file".
- **Sprint use, same day (519):** terra:med drained ~1 Codex 5h point per large turn
  (up to 620k new tokens), so it stayed the developer for a full feature; luna turns
  (40–65k new) stayed below 1 point. flash37 stopped on an agy quota error the usage
  display did not predict (516). Measured drain per model is now in
  `harnez stats --agents`. Details: [sprint 519 retro](studies/2026-09-24-sprint-519-agent-stats.md).

## 2026-09-23 web-research snapshot

Method: one `codex:luna:low` advisor per configured model (11 in parallel, 27–55s each, ~40k
new tokens each) web-searched Go, TUI, LLM tool/agent development and automation/CLI
performance, ignoring web, JS/TS and Python. Sources were not opened by the host, so treat scores
as agent-reported.

| Model | Go | TUI | Tools/agents | Automation/CLI | List cost |
|---|---|---|---|---|---|
| codex:luna (gpt-6-luna) | ok | weak | ok | weak (Terminal-Bench 13%) | cheapest |
| codex:sol (gpt-6-sol) | ok | ok | strong | strong (TB2.1 83%) | mid |
| codex:astra | strong? | ok | strong | strong (tops TB4.0) | flagship |
| codex:terra | ok | ok | strong | strong (TB2.1 87%) | mid |
| claude:haiku | ok | weak | ok if bounded | weak on long runs | low |
| claude:sonnet | ok | ok | strong | strong | mid |
| claude:opus | strong | thin | ok | strong (TB4.0 66%) | premium |
| agy:flash37 | ok | ok | ok | strong if bounded (TB2.1 86%) | listed low |
| agy:flash38 | ok | ok | ok | strong (TB2.1 89%), >13s first token | listed low |
| agy:sonnet (4.6) | ok | ok | strong | strong | mid |
| agy:opus (4.6 thinking) | strong | ok | strong | strong | high |

Findings:

- **No Go- or TUI-specific benchmark exists for any model.** Those columns are inferred from
  general coding and terminal benchmarks; our own sprint evidence (`ModelAdvisoryEval.md`)
  outranks them.
- **flash37/flash38 are frontier-grade, not mechanical workers.** Their Terminal-Bench 2.1 scores
  match terra/sol. List price is low, but subscription cost feels high in practice; ticket 507
  (quota snapshots per agent turn) is meant to measure real subscription cost.
- **Research agents disagree on identity and price.** The astra agent found "GPT-6 Astra", not
  `gpt-5.6-astra`; sol and terra agents gave conflicting prices. The agent was right: Codex's
  `~/.codex/models_cache.json` offers only `gpt-6-astra`, and the spec was fixed on
  2026-09-23. Check model names in `spec/agent.yaml` against the provider's own model list
  (Codex: `slug` entries in `models_cache.json`), not against docs or web research.
- **One agent drifted off task.** The agy:opus researcher also audited repo code and ticket 500:
  the advisor role preamble invites repo exploration, so research prompts should say
  "web research only, do not inspect the repository".

## 2026-09-23 SQL research snapshot (SQLite, SQL analytics, making the right choices)

Same method, prompts marked "web research only" and run from a scratch dir. No agent
drifted into a repository this time. Python/pandas excluded.

| Model | SQLite | SQL analytics | Right choices (schema, metrics, ambiguity) | Tag |
|---|---|---|---|---|
| claude:opus | ok | strong (Spider 2.0 70%, BIRD ~69–70%) | ok | SQL+ |
| agy:opus (4.6) | thin | ok (BIRD ~69–70%, LiveSQLBench #2) | weak/thin | SQL~ |
| codex:astra | thin | ok (BIRD subset 66%) | ok, indirect (asks when underspecified) | SQL~ |
| codex:terra | thin | ok (Tinybird, many first-try exact) | thin | SQL~ |
| claude:sonnet | thin | ok (Tinybird leader, exactness only ~56/100) | weak (literal-assumption errors) | SQL~ |
| agy:sonnet (4.6) | thin | ok, agent-based scores only | thin | SQL~ |
| claude:haiku | ok (Anthropic SQLite guide) | ok (BIRD 51%) | weak/thin | SQL~ |
| agy:flash37 | thin | ok, Gemini 3 (not 3.7) data | thin | SQL~ |
| agy:flash38 | thin | ok (small practitioner test 76/100) | thin | SQL~ |
| codex:sol | no data | no data | no data | SQL~ (provisional) |
| codex:luna | no data | no data | no data | SQL- (unmeasured) |

Findings:

- **No model has SQLite-specific evidence** (type affinity, JSON1, `EXPLAIN QUERY PLAN`,
  indexing). Benchmarks use SQLite as a runtime, but don't score its quirks.
- **"Making the right choices" is unmeasured everywhere.** Benchmarks assume one gold
  query; ambiguity handling, metric choice and double counting from fan-out joins are not
  scored. Even top models reach only ~56–65% exactness on analytics prompts, so every model
  needs review against a known result.
- **Only opus has strong evidence.** Luna's SQL- means "no data", not "proven weak"; since
  luna is the `try` model, a small repo-local SQLite canary (known schema, known answers,
  a fan-out trap) is the cheapest way to get real evidence for luna, sol and flash.
- Research cost: ~11 × 30–50k new tokens, 24–49s each.

## Earlier assessment (pre-GPT-6 lineup, user notes)

  Comparing the latest generation—Claude Sonnet 5, Gemini 3.7 Flash, and OpenAI’s GPT-5.5 / 5.6 tier   
  family (Sol, Terra, Luna)—explains why Sol often exhibits unexpected behavior compared to the other  
  two:                                                                                                 
  ──────                                                                                               
  ### 1. The Head-to-Head Comparison                                                                   
                                                                                                       
   Attribute         │ Claude Sonnet 5   │ Gemini 3.7 Flash  │ GPT-5.5/5.6 Sol    │ GPT-5.5/5.6 Terra…
  ───────────────────┼───────────────────┼───────────────────┼────────────────────┼────────────────────
   Primary           │ Adaptive Agentic  │ High-Throughput   │ Heavy Deep         │ Standard Workhorse
   Philosophy        │ Precision         │ Intelligence      │ Reasoning / STEM   │ / Routing
   Reasoning         │ Granular Adaptive │ Direct generation │ Heavy autonomous   │ Lightweight direct
   Approach          │ Thinking (Low →   │ + Fast Internal   │ Chain-of-Thought   │ generation
                     │ X-High)           │ Planning          │ deliberation       │
   Context Window    │ 1M Tokens         │ 1M Tokens         │ 256k – 1M Tokens   │ 128k – 256k Tokens
   Developer         │ Surgical diffs,   │ Low latency,      │ Tends to over-     │ Predictable, but
   Ergonomics        │ strict            │ predictable       │ abstract, over-    │ lower ceiling on
                     │ instruction       │ output,           │ refactor, or       │ difficult logic
                     │ adherence, low    │ straightforward   │ hallucinate edge   │
                     │ drift             │ APIs              │ cases              │
   Sweet Spot        │ End-to-end coding │ Real-time apps,   │ Complex math       │ High-volume
                     │ loops,            │ large-context     │ proofs, exploit    │ pipelines, simple
                     │ refactoring, CLI  │ parsing, UI/web   │ analysis, deep     │ summaries, fast
                     │ agents            │ gen               │ algorithmic design │ triage
  ──────
  ### 2. Why Sol Feels "Weird" Compared to Sonnet 5 and Gemini 3.7 Flash
  
  1. Over-Engineering & Speculative Divergence (Sol):
      • Sol is tuned for deep deliberation, cybersecurity verification, and heavy STEM reasoning.      
      • When given standard software engineering tasks, it often consumes its reasoning budget         
      exploring extreme edge cases. This leads to unwanted refactoring, over-abstracted boilerplate, or
      solutions tailored to hypothetical failure modes you never asked for.
  2. Adaptive Control vs. Unbounded Deliberation (Sonnet 5):
      • Sonnet 5 uses Adaptive Thinking levels (Low to X-High). It remains grounded in the user's      
      prompt and adheres tightly to existing codebase conventions without "runaway reasoning."         
      • Its RLHF emphasizes surgical, minimal diffs, making it significantly more predictable in       
      agentic loops.
  3. Pragmatic Directness (Gemini 3.7 Flash):
      • Gemini 3.7 Flash is tuned as an agile workhorse (scoring high on benchmarks like DeepSWE and   
      WebDev Arena).
      • It solves tasks without spending hidden tokens second-guessing the prompt, resulting in clean, 
      direct solutions with minimal behavioral drift.
  
  ──────
  ### 3. Practical Usage Summary
  
  • Default to Claude Sonnet 5: When running multi-step agentic workflows, refactoring existing        
  repositories, or when you need tight adherence to file conventions.
  • Default to Gemini 3.7 Flash: When you need rapid first-pass code generation, UI/web layout builds, 
  large-document digestion, or low-latency agent iterations.
  • Use GPT-5.5/5.6 Sol selectively: Reserved for isolated, difficult mathematical, cryptographic, or  
  algorithmic problems where extensive exploratory reasoning is explicitly desired. For standard daily 
  workflows, Terra provides a more stable experience than Sol.
