# 670 — lean-sprint handoff: every prohibition names the allowed alternative; API mappings as full tables

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: docs/commands/lean-sprint.md, 545; loom docs/LeanSprints.md (Multi-repo migration 2026-10-01)

---

/goal Make lean-sprint handoffs state the allowed alternative with every prohibition and give semantic
API mappings as full tables, or stop and report when blocked on a user decision.

## 1. Problem & Motivation
In the loom 235 migration (8 repos, one luna developer each) two of the review findings came from the host's
own handoff wording, not from the developers:
- "never hand-edit go.sum" was read as "do not touch go.sum": cati's commit left `go.sum` stale.
- "a used key returns Handled" made harnez map the old bool `true` (= quit) to `loom.Handled()`; q no longer quit.
Both commits passed their test suites; only diff review caught them.

## 2. Technical Specification / Findings
- Prohibitions get read literally and broadly. "Don't X" needs "do Y instead" (e.g. "let `go mod tidy`
  update go.sum").
- A partial hint for a semantic mapping steers wrong. Give the full old→new table as fact.
- Lean-sprint skill source: `docs/commands/lean-sprint.md` (handoff checklist in §1).

## 3. Implementation & Verification Plan
- Add both rules to the lean-sprint handoff checklist (and the developer prompt if it has a constraints section).
- Keep it to two short bullets; no wording template.
