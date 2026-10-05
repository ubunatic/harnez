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

Reassessed 2026-10-05 with two Artificial Analysis Intelligence Index chart
captures. **Proposal: keep all 11 costs.** Current values and tiers are from
`spec/agent.yaml`. COST remains plan-quota share per typical turn (Luna = 1,
Astra = 100), per [Models.md](../docs/Models.md) and
[ModelResearch.md, step 3](../docs/ModelResearch.md#3-reconcile).

Sources: **P** = [Preuve article](https://preuve.ai/blog/ai-coding-models-statistics-2026),
fetched live; **AA** = [Artificial Analysis report](../docs/studies/2026-10-05-artificial-analysis-cost-report.md);
**AA-chart** = two captures dated 2026-10-05: `/home/uwe/Downloads/Intelligence Index vs- Cost per Intelligence Index Task (5 Oct '26).png`
and `/home/uwe/Downloads/Intelligence Index vs- Cost per Intelligence Index Task (5 Oct '26)(1).png`.
Comparability is to Harnez subscription quota per turn: direct = same measure;
indirect = model/effort benchmark signal but different API task-cost measure,
route, or tier; none = no relevant model data.

| Spec | Current cost | Proposed cost | Source(s) | Comparability | Reason |
|---|---:|---|---|---|---|
| `codex:luna` | 1 | keep | P; AA; AA-chart | indirect | Chart labels low through max; API task cost is not low-tier quota. |
| `codex:sol` | 20 | keep | P; AA; AA-chart | indirect | Low through max labeled; task cost and benchmark remain API-based. |
| `codex:astra` | 100 | keep | P; AA; AA-chart | indirect | Low through max labeled; no quota measurement or numeric chart labels. |
| `codex:terra` | 26 | keep | P; AA | indirect | Max Index API task cost does not measure configured quota. |
| `claude:haiku` | 12 | keep | P | indirect | Haiku 4.5 Index API task cost; alias version/tier unverified. |
| `claude:sonnet` | 15 | keep | P; AA; AA-chart | indirect | Low/medium through max labeled with fallback; quota unmeasured. |
| `claude:opus` | 20 | keep | P; AA; AA-chart | indirect | Low/medium through max labeled with fallback; quota unmeasured. |
| `agy:flash37` | 4 | keep | AA-chart | indirect | Gemini 3.7 Flash high appears; no numeric labels or quota data. |
| `agy:flash38` | 4 | keep | P; AA-chart | indirect | Gemini 3.8 Flash high appears; no numeric labels or quota data. |
| `agy:sonnet` | 16 | keep | P | indirect | Claude Sonnet evidence does not identify an agy route or quota. |
| `agy:opus` | 32 | keep | P; AA | indirect | Claude Opus evidence does not identify an agy route or quota. |

Chart values explicitly readable:

- Both images label GPT-6 Luna at low, medium, high, xhigh, max, and
  non-reasoning; GPT-6.1 Sol and GPT-6 Astra at low, medium, high, xhigh, max.
- Both label Claude Sonnet 5.5 and Opus 5.5 at low, medium, high, xhigh, max,
  each “with fallback”; Gemini 3.7 Flash and Gemini 3.8 Flash at high.
- Neither chart prints numeric Intelligence Index scores or USD task costs
  alongside the points. Those values are unlabeled, so none are estimated from
  plotted positions. The captures therefore add effort labels, not reproducible
  numeric ratios. No proposed value changes; no ratio method applies.

Unverified or non-comparable findings:

- No source or chart measures subscription quota per typical turn; converting
  API task costs to the Luna/Astra scale would require a new matched-task quota
  method and is not reproducible from these captures.
- Although low/medium chart configurations are closer to Harnez effort tiers,
  the captured charts still use API cost per Index task, and Claude labels include
  fallback. Claude routes do not establish agy-specific differences.
- Charts show Gemini 3.7 Flash high, changing that row from none to indirect;
  they do not expose a numeric point value. Gemini 4 Argon is a different model.
- Claude model names in Harnez are floating aliases; their current resolution
  remains unchecked. The charts label Claude 5.5, not agy Sonnet/Opus.
- `docs/Models.md` retains the historical Sol quota estimate 50 for GPT-5.6
  Sol; the current spec selects GPT-6.1 Sol at 20. Neither chart supplies quota
  evidence to revise it.

M1 verification: reviewed both images and the ticket-only diff; no tests run.
Ticket stays open pending user approval, edits, or rejection.
