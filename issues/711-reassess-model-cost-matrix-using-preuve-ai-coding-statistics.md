### M1 result (approved 2026-10-05)
M1 delivered: proposal table and chart estimates (section 5). The user decided
that one cost per model is too coarse: cost varies with effort tier about as much
as with model. Approved direction: a per-tier effort matrix anchored at
`codex:luna:low = 1` (the old `astra = 100` anchor is dropped).

### M2 — Effort matrix in spec and listing
**Pre-Work / Required Refinements:**
- Section 5 misreads Claude Opus 5.5 (low, with fallback) as $1.13 / 46; that
  point is Sonnet 5.5 (high). Correct value from both charts: ~$0.55 / 42.
  Mark Opus medium ($1.83 / 53) as uncertain (crossing leader lines).
- `docs/Models.md` still carries the historical Sol estimate 50; align it.

Implementation:
- Allow a per-tier cost in `spec/agent.yaml`, following the existing `use_med`
  override pattern or a cleaner per-tier form; keep the schema in sync and
  keep a single `cost` valid for models without tier data (Spec.md: no Go
  defaults duplicating spec values).
- `harnez agent models` shows the tier's cost on each `:low` / `:med` row; the
  legend states the new anchor (luna:low = 1) and that values are estimates.
- Matrix tiers: **low, med and high** only (user decision 2026-10-05: models
  support more levels, but harnez rarely uses them; xhigh/max/non-reasoning stay
  out of the matrix). Add `:high` rows to the listing where the provider
  supports that effort.
- Initial codex values (estimated from AA charts, 5 Oct 2026, API cost ratio to
  luna:low within the ChatGPT plan): luna low 1 / med 4; sol low 28 / med 45;
  astra low 180 / med 337. High: estimate luna, sol and astra (high) from the
  chart the same way and record the reading in section 5; the Astra labels
  near $1.5-$2.5 have crossing leader lines, so state which dot you used. Terra, Claude and Gemini keep one value for both
  tiers (separate plan quotas, no matched data); rescale nothing else unless
  the new anchor requires it, and say so if it does.
- Update `docs/Models.md` (and `docs/ModelResearch.md` for the method change).
- Tests: unit tests for per-tier parsing and listing, including a model with
  only a single `cost`, a tier override, and an unknown tier.

### M3 — Exact per-tier data
**Pre-Work / Required Refinements (host review of M2, 2026-10-05):**
- Astra high is misread. In both charts the "GPT-6 Astra (xhigh)" leader line
  points to the ~$2.3 / 52 dot; the "high" label belongs to the half-hidden dot
  at ~$1.75 / 51.5 (chart order is then monotonic: low $0.82, med $1.55, high
  $1.75, xhigh $2.3, max $3.26). Set `codex:astra` high to 380
  ($1.75 / $0.0046) unless the user decides otherwise, and fix section 5.
Replace chart estimates with exact numbers. Canary first: probe whether the
Artificial Analysis model pages or data API expose exact cost per task by
effort (an API key may be needed from the user; stop and ask). Data review
only (user decision 2026-10-05): use published data, run no new measurements,
benchmarks or quota trials. Record source and date per value; close with
`harnez issues close 711 "<real outcome>"`.

# 711 — Reassess model cost matrix using Preuve AI coding statistics

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [Model assessment](../docs/Models.md), [Model research process](../docs/ModelResearch.md), [Preuve AI coding model statistics](https://preuve.ai/blog/ai-coding-models-statistics-2026), [Artificial Analysis cost report](../docs/studies/2026-10-05-artificial-analysis-cost-report.md), `spec/agent.yaml`

---

## 1. Problem & Motivation
Reassess the model cost matrix using the comparisons and data in the
[Preuve AI coding model statistics article](https://preuve.ai/blog/ai-coding-models-statistics-2026).
The current matrix uses measured subscription quota per typical turn; determine
whether the article provides comparable evidence that warrants changing its
values or cost methodology.

## 2. Technical Specification / Findings
The article's measurements may use different workloads, providers, pricing
bases, or definitions of cost. Record comparability and limitations before
applying any values. Preserve the existing luna/astra scale unless the research
supports and justifies a change.

Second source (added 2026-10-05): the
[Artificial Analysis cost report](../docs/studies/2026-10-05-artificial-analysis-cost-report.md)
lists USD API cost per benchmark task. Its own conclusion is that these values do
not convert to the quota scale; treat it as a supporting signal, not a direct input.

## 3. Implementation & Verification Plan
/goal Review the linked article against the current cost matrix and update the
matrix and supporting rationale where its evidence is comparable; document
inconclusive or non-comparable findings without forcing a change, or stop and
report when blocked on user input or denied permission. Verify model tables
and generated/listed cost values remain consistent.

## 4. Milestones

### M1 — Proposal table (no spec changes)
Research both sources against the current `cost` values in `spec/agent.yaml` and
`docs/Models.md`. Append to this ticket a section `## 5. M1 Cost Proposal` with
one table row per model in `spec/agent.yaml`: spec, current cost, proposed cost
(or "keep"), source(s), comparability (direct / indirect / none), and a one-line
reason. Below the table list unverified or non-comparable findings. Commit only
the ticket (`docs(issues): 711 M1 cost proposal`). Do not edit `spec/agent.yaml`,
`docs/Models.md`, or any code in M1.

**Approval gate:** the user approves, edits, or rejects each proposed change
before M2 starts.

### M2 — Apply approved changes
Apply only the approved values to `spec/agent.yaml`, update the rationale in
`docs/Models.md` (and `docs/ModelResearch.md` if the method changes), run the
tests that cover model listing, check `harnez agent models` shows the new values,
commit, and close with `harnez issues close 711 "<real outcome>"`.

## 5. M1 Cost Proposal

Reassessed 2026-10-05 using both Artificial Analysis Intelligence Index chart
captures. Current costs come from `spec/agent.yaml`; the scale is subscription
quota per typical turn, Luna = 1 and Astra = 100, per [Models.md](../docs/Models.md)
and [ModelResearch.md, step 3](../docs/ModelResearch.md#3-reconcile).

Sources: **P** = [Preuve article](https://preuve.ai/blog/ai-coding-models-statistics-2026);
**AA** = [Artificial Analysis report](../docs/studies/2026-10-05-artificial-analysis-cost-report.md);
**AA-chart A/B** = the two 2026-10-05 captures:
`/home/uwe/Downloads/Intelligence Index vs- Cost per Intelligence Index Task (5 Oct '26).png`
and the same name ending `(1).png`. All chart figures below are **estimates from
chart position** (API USD per Index task; roughly ±10% cost and ±1 Index point),
read against the log-cost and linear-Index axes, and reconciled across both
images. Harnez tier is the spec's default low unless stated. Comparability:
direct = same quota measure; indirect = API benchmark evidence; none = no relevant
source data. Within Codex's shared plan only, same-tier API cost ratios are a
plausible quota-use proxy; Claude Pro and Google Pro have separate quota pools,
so cross-vendor ratios are not used to propose their quota costs.

### Chart estimates

| Model/configuration | Chart A: cost, Index | Chart B: cost, Index | Reconciled estimate |
|---|---:|---:|---:|
| GPT-6 Luna (low) | $0.0046, 21 | $0.0046, 21 | **Estimated from chart position:** $0.0046, 21 |
| GPT-6 Luna (medium) | $0.0185, 29 | $0.018, 29 | **Estimated from chart position:** $0.018, 29 |
| GPT-6.1 Sol (low) | $0.124, 42 | $0.129, 42 | **Estimated from chart position:** $0.13, 42 |
| GPT-6.1 Sol (medium) | $0.204, 47 | $0.205, 47 | **Estimated from chart position:** $0.205, 47 |
| GPT-6 Astra (low) | $0.83, 45 | $0.82, 45 | **Estimated from chart position:** $0.83, 45 |
| GPT-6 Astra (medium) | $1.54, 49 | $1.56, 49 | **Estimated from chart position:** $1.55, 49 |
| GPT-6 Luna (high) | ~$0.030, 33 | ~$0.030, 33 | **Estimated from chart position:** ~$0.030, 33 |
| GPT-6.1 Sol (high) | ~$0.32, 50 | ~$0.32, 50 | **Estimated from chart position:** ~$0.32, 50 |
| GPT-6 Astra (high) | ~$2.3, 52 | ~$2.3, 52 | **Estimated from chart position:** ~$2.3, 52 |
| Claude Sonnet 5.5 (low, with fallback) | $0.43, 35 | $0.44, 35 | **Estimated from chart position:** $0.44, 35 |
| Claude Sonnet 5.5 (medium, with fallback) | $0.60, 40 | $0.59, 41 | **Estimated from chart position:** $0.60, 41 |
| Claude Sonnet 5.5 (high, with fallback) | ~$0.55, 42 | ~$0.55, 42 | **Estimated from chart position:** ~$0.55, 42 |
| Claude Opus 5.5 (low, with fallback) | ~$0.55, 42 | ~$0.55, 42 | **Estimated from chart position:** ~$0.55, 42 |
| Claude Opus 5.5 (medium, with fallback) | $1.83, 53 | $1.83, 53 | **Estimated from chart position, uncertain:** $1.83, 53 (leader lines cross) |
| Gemini 3.7 Flash (high) | $0.93, 39 | $0.93, 39 | **Estimated from chart position:** $0.93, 39 |
| Gemini 3.8 Flash (medium) | ~$0.93, ~39 (overlaps 3.7 high) | ~$0.93, ~39 (overlaps 3.7 high) | **Estimated from chart position:** ~$0.93, ~39 |
| Gemini 3.8 Flash (high) | $1.25, 40 | $1.24, 40 | **Estimated from chart position:** $1.24, 40 |

### Matched-tier Codex ratios

These use the reconciled API costs above and Luna = 1 as the reference. Astra=100
is shown separately; the chart-derived anchors do not agree with each other.

- Low: Sol/Luna = $0.13/$0.0046 = **28.3**; Astra/Luna = $0.83/$0.0046 =
  **180.4**. Against Astra=100 instead: Luna = 100×$0.0046/$0.83 = **0.55**;
  Sol = 100×$0.13/$0.83 = **15.7**; Astra = **100**.
- Medium: Sol/Luna = $0.205/$0.018 = **11.4**; Astra/Luna =
  $1.55/$0.018 = **86.1**. Against Astra=100 instead: Luna =
  100×$0.018/$1.55 = **1.16**; Sol = 100×$0.205/$1.55 = **13.2**;
  Astra = **100**.

The default-low ratio gives concrete provisional candidates Sol ≈28 and Astra
≈180 (cost = same-tier API task cost ÷ Luna API task cost × Luna cost 1). The
medium check instead gives Sol ≈11 and Astra ≈86. This effort sensitivity and
the conflicting Luna-anchored versus Astra-anchored results prevent treating
either as a confirmed quota value. No estimate differs from its current value
by more than 2×; Astra's low-tier candidate is 1.8× current. Flag both rows for
review pending approval and matched plan-quota measurements before M2.

### Revised proposal

| Spec | Current cost | Proposed cost | Source(s) | Comparability | Reason |
|---|---:|---|---|---|---|
| `codex:luna` | 1 | keep (anchor) | AA-chart | indirect | Reference row; estimates on Astra=100 give 0.55 low / 1.16 medium. |
| `codex:sol` | 20 | review: ~28 low-tier candidate | AA-chart | indirect | Low ratio .13/.0046×1=28.3; medium ratio .205/.018×1=11.4. |
| `codex:astra` | 100 | review: ~180 low-tier candidate | AA-chart | indirect | Low ratio .83/.0046×1=180.4; medium gives 86.1; conflicts with Astra=100 anchor. |
| `codex:terra` | 26 | keep | P; AA | indirect | No matched low/medium chart point; API Index cost does not measure quota. |
| `claude:haiku` | 12 | keep | P | indirect | Haiku API evidence lacks a matched quota measure and alias/tier check. |
| `claude:sonnet` | 15 | keep | P; AA; AA-chart | indirect | Claude Pro quota differs; chart's low/medium points include fallback. |
| `claude:opus` | 20 | keep | P; AA; AA-chart | indirect | Claude Pro quota differs; chart's low/medium points include fallback. |
| `agy:flash37` | 4 | keep | AA-chart | indirect | Gemini 3.7 high is charted; Google Pro quota differs from Codex. |
| `agy:flash38` | 4 | keep | P; AA-chart | indirect | Gemini 3.8 medium/high is charted; Google Pro quota differs from Codex. |
| `agy:sonnet` | 16 | keep | P | indirect | Claude route data cannot establish the agy route's quota consumption. |
| `agy:opus` | 32 | keep | P; AA | indirect | Claude route data cannot establish the agy route's quota consumption. |

Unverified: chart points are approximate; Gemini 3.8 medium overlaps Gemini 3.7
high, so the shared coordinate is especially uncertain. Claude points include
fallback; the charts do not isolate provider-specific agy usage. The two Codex
anchors cannot both be reproduced by the estimated task-cost ratios. No row is
over 2× off current; Codex Sol/Astra are marked review for effort/anchor
disagreement, not as confirmed corrections. The cost point formerly assigned to
Claude Opus 5.5 low ($1.13 / 46) is actually Sonnet 5.5 high; Opus low is about
$0.55 / 42. Opus medium is uncertain because its leader lines cross.

### M2 data review and applied matrix (2026-10-05)

The added high-tier readings are estimated from chart position and reconciled
across both captures. On Chart A and B, Luna high is about $0.030 / Index 33,
Sol high about $0.32 / 50, and Astra high about $2.3 / 52. Astra's label leaders
cross in the $1.5–$2.5 cluster: the selected high point is the black dot near
$2.3 / 52 that the `GPT-6 Astra (high)` leader targets in both images. The
neighboring dot near $1.5 / 54 is xhigh; the distinct point around $3.3 / 52 is
max. This label-to-dot match is more consistent across captures than reading the
crossing leader paths alone.

The approved method divides each Codex API task cost by Luna-low ($0.0046) and
rounds to the nearest integer. Arithmetic: Luna high `$0.030 / $0.0046 = 6.52 →
7`; Sol high `$0.32 / $0.0046 = 69.57 → 70`; Astra high `$2.3 / $0.0046 = 500`.
The low/medium values were approved in M1: Luna 1/4, Sol 28/45, Astra 180/337.
This API ratio is used only within the shared ChatGPT plan as a proxy for quota
use. Claude Pro and Google Pro rows keep their existing single values across
effort tiers because their quota pools differ; Terra and agy-routed Claude rows
also remain unchanged.

| Spec | Stored `cost` fallback | Low | Med | High |
|---|---:|---:|---:|---:|
| `codex:luna` | 1 | 1 | 4 | 7 |
| `codex:sol` | 28 | 28 | 45 | 70 |
| `codex:astra` | 180 | 180 | 337 | 500 |
| `codex:terra` | 26 | 26 | 26 | 26 |
| `claude:haiku` | 12 | 12 | 12 | 12 |
| `claude:sonnet` | 15 | 15 | 15 | 15 |
| `claude:opus` | 20 | 20 | 20 | 20 |
| `agy:flash37` | 4 | 4 | 4 | 4 |
| `agy:flash38` | 4 | 4 | 4 | 4 |
| `agy:sonnet` (no effort) | 16 | 16 | — | — |
| `agy:opus` (no effort) | 32 | 32 | — | — |

The listing emits low/med/high rows for all effort-capable providers and one low
row for `agy:sonnet` and `agy:opus`. M2 is complete; exact data review remains in
M3. No new benchmarks or measurements were run. Implementation is committed as
`f8303d4c`, with research-doc updates in `92f63d1f`.

Verification: `make install` passed. The pre-change `HTO=0 make test-q1` failed
`TestAgentDefaultModelIsMarkedOnce`, `TestAgentStartDefaultModelLine`,
`TestFlash38EscalationGuidanceAndLeanSprintDeveloperPreference`, and
`TestEmbeddedAgentSpecLoadsAndResolves`. After adding high rows,
`TestAgentDefaultModelIsMarkedOnce` passes; `TestAgentStartDefaultModelLine`
still fails because its old assertion expects `codex:luna:low` while the spec
default is `codex:luna:high`. The two flash38/default-spec mismatches remain.
Those existing assertions were left unchanged. The new tier-cost, listing, and
fallback tests pass. The ticket remains open for M3.
