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

Reviewed 2026-10-05; pending user approval. **Proposal: keep all 11 costs.**
Current values come from `spec/agent.yaml`, including its default `low` tiers;
`keep` means retain that value, not newly validate it. COST remains subscription
quota per typical turn (ChatGPT Plus / Claude Pro / Google Pro), Luna = 1,
Astra = 100, following [Models.md](../docs/Models.md) and
[ModelResearch.md, step 3](../docs/ModelResearch.md#3-reconcile).

Sources: **P** = [Preuve article](https://preuve.ai/blog/ai-coding-models-statistics-2026),
fetched live successfully (updated 2026-10-01); **AA** =
[Artificial Analysis cost report](../docs/studies/2026-10-05-artificial-analysis-cost-report.md).
Dollar figures below are source-reported API USD per task: **coding** means
Coding Agent Index; **Index** means Intelligence Index. They are separate measures.
Comparability: **direct** requires comparable subscription-quota evidence;
**indirect** supplies model/family API evidence with unresolved configuration or
billing differences; **none** means neither source supplies data for that model.

| Spec | Current cost | Proposed cost | Source(s) | Comparability | Reason |
|---|---:|---|---|---|---|
| `codex:luna` | 1 | keep | P | indirect | Max coding $0.18 does not measure low-tier quota. |
| `codex:sol` | 20 | keep | P; AA | indirect | Low coding $0.50; max Index $0.72; neither measures quota. |
| `codex:astra` | 100 | keep | P; AA | indirect | Max coding $7.47 / Index $3.26 cannot recalibrate quota. |
| `codex:terra` | 26 | keep | P | indirect | Terra max Index $1.40 differs from low-tier quota. |
| `claude:haiku` | 12 | keep | P | indirect | Haiku 4.5 Index $0.28; alias version/tier unverified; API only. |
| `claude:sonnet` | 15 | keep | P | indirect | Sonnet 5.5 medium coding $0.62 differs from configured low-tier quota. |
| `claude:opus` | 20 | keep | P; AA | indirect | Opus 5.5 max coding $13.04 / Index $5.98; effort/fallback differ. |
| `agy:flash37` | 4 | keep | — | none | No data for Gemini 3.7 Flash in either source. |
| `agy:flash38` | 4 | keep | P | indirect | Flash 3.8 high Index $1.24 does not measure low-tier quota. |
| `agy:sonnet` | 16 | keep | P | indirect | Sonnet evidence does not establish an agy-specific quota difference. |
| `agy:opus` | 32 | keep | P; AA | indirect | Opus evidence does not establish an agy-specific quota difference. |

Unverified or non-comparable findings:

- Neither source measures plan-quota share per typical turn; no USD-to-quota
  conversion is proposed. A change needs matched-task quota measurements with
  recorded plan, model version, effort, cache conditions, route, and sample size.
- AA labels Opus 5.5 **max with fallback**; P calls it **max**. Their matching
  $5.98 Index figure does not verify identical routing. Neither matches Harnez's
  Claude low/med configurations or agy's `effort: false` route.
- Claude `haiku`/`sonnet`/`opus` are floating aliases; their current resolutions
  were not checked. agy's Sonnet/Opus names explicitly select 5.5. Preserve
  Claude/agy costs 15/16 and 20/32 provisionally; no source verifies those gaps.
- P attributes its benchmark costs to Artificial Analysis, so P and AA are not
  independent replications. P's data is dated 2026-10-01; AA's saved HTML has no
  capture timestamp. No costs were inferred from chart positions.
- Gemini 4 Argon is not Flash 3.7 or 3.8; do not substitute its costs. AA's
  selected top-ten table omits several configured models; P's expanded table
  supplies Terra, Haiku 4.5, and Flash 3.8, but neither supplies Flash 3.7.
- `docs/Models.md` retains the historical Sol quota estimate 50, based on
  GPT-5.6 Sol with GPT-6 Sol unmeasured; the current spec selects GPT-6.1 Sol at
  20. This history does not justify reverting the current value in M1.

M1 verification: checked all 11 model keys/current costs against the spec and
reviewed the ticket-only diff. No tests run (documentation only). Ticket stays
open; M2 requires the user's approval, edits, or rejection of this proposal.
