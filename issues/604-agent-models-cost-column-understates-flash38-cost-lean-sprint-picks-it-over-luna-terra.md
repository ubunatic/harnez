# 604 — agent models cost column understates flash38 cost; lean-sprint picks it over luna/terra

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 451 (default to low-cost models), 453 (cost guidance), 603 (quota blindness)

---

## 1. Problem & Motivation
`harnez agent models` lists `agy:flash38:*` at COST 4 (× luna), which is cheaper than
`codex:terra` (26). The user considers flash38 expensive and wants luna or terra as
developers. In a neus `/lean-sprint 19` run the host picked `agy:flash38:med` from this
table because its TUI rating (`~`) beat luna's (`-`) and its listed cost was low.

## 2. Technical Specification / Findings
- The COST figure for flash38 in the model spec does not match its real cost (for
  example its token use per goal or its quota drain).
- The lean-sprint skill says "pick the model from `harnez agent models`; e.g. `--model luna`"
  but does not say luna/terra are the preferred developers or that agy models should be avoided.

## 3. Implementation & Verification Plan
- Correct flash38's cost in the model spec (spec/agent.yaml or equivalent) to reflect real
  cost, or add an "avoid"/escalation-only note to its USE column.
- State the developer preference (luna first, terra when stronger judgment is needed) in the
  lean-sprint skill and the `agent models` USE text.
- Verify: `harnez agent models` shows the corrected cost/usage for flash38.

## Fix 2026-09-27

`34d79de`, `4497d91`: flash38 USE text says escalation-only/avoid for developer work, developer role removed (reviewer, advisor remain); lean-sprint skill prefers luna, then terra, no agy developers. Cost kept at 4: no measured agy cost data. `harnez apply` done. Awaiting neus feedback.
- neus (2026-09-27): current sprint 019 already runs on codex:luna:med; will confirm the model pick on its next sprint (its `harnez init` awaits user approval).
