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

