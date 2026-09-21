# 467 — Recommend a read-only plan-first initial prompt in sprint skills and docs, without a template

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Workflow / Docs
**Related**: [176](176-structured-capped-subagent-completion-report-contract.md), [288](288-add-fresh-codex-skill-lean-fresh-handoff-sprint-using-codex-cli-instead-of-a-claude-subagent.md), `docs/AgenticLoop.md`, `/lean-sprint`, `/sprint`

---

## Problem

Plan-first ("reply with a rough plan before editing") is only a sentence in a worker prompt.
In one-shot `harnez agent start` the 462 worker skipped it and edited straight away, so the host
never got to steer the plan before write authority.

## /goal

The sprint skills and related docs recommend that a worker's *initial* prompt asks for a
**read-only plan first** (no edits until the host approves, then `resume` grants write authority).
Recommend only; provide **no prompt template**. Agents have their own best practices, and the
user wants to observe and compare them: first prompts are already stored in the telemetry DB.

## Notes

- Touch `/lean-sprint`, `/sprint` and `docs/AgenticLoop.md` (which has an uncommitted user edit
  on steered escalation; coordinate, don't overwrite).
- Optional follow-up: a stats view of stored first prompts per agent and whether they asked for a
  plan, to observe what works.
- Re-verify against live code and docs before starting.

## Delivered (2026-09-21)

Recommendation added, without a template, to `docs/commands/lean-sprint.md`, `docs/commands/sprint.md` and `docs/AgenticLoop.md` (Invariant 7). The optional stats view of stored first prompts is not built; file separately if wanted.
