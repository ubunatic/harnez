# 1204 — Add Claude Fable 5.2 as a harnez agent model

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 755 (added Haiku 5.5, same pattern), 1203 (official model names), 983 (model picks)

---

## 1. Problem & Motivation

Owner request (2026-11-10): support Claude Fable 5.2 as a harnez model. `harnez agent models`
lists Claude Haiku, Sonnet and Opus but no Fable entry, and `spec/` has no mention of Fable,
so `harnez agent start --model fable` cannot dispatch it.

## 2. Goal

/goal `harnez agent start --model claude:fable[:<tier>]` dispatches Claude Fable 5.2
(official ID `claude-fable-5-2`) and `harnez agent models` lists it, with every value coming
from `spec/agent.yaml`; stop and report when a needed value (price, effort support, roles) has
no source and the choice is the owner's. On a loop or no progress, clear the goal and write a
failure report here.

## 3. Acceptance

- A `claude:fable` entry in `spec/agent.yaml` (schema-valid) modeled on the 755 Haiku entry.
- `harnez agent models` shows `claude:fable:low|med|high`; a dry run or resolver test resolves
  `--model fable` to it.
- No live call to Fable (owner, 2026-11-10). Verify by resolver/dry-run tests only.
- Cost values are estimates, marked as such in the spec entry's comment or source field, with
  the basis stated (e.g. relative to the existing Opus/Sonnet cost values and any published price).

## Decision

- 2026-11-10 owner: do not call the model; add it and estimate the costs.

## 4. Notes / Unverified

- The ID `claude-fable-5-2` comes from Claude Code's model list; whether the Claude Code CLI
  takes `fable` as a `--model` alias is unverified (check `claude --help` or one live run).
- Cost, effort support, compaction threshold, skills and roles are unknown; mark them `?`
  until measured, or ask the owner. Whether agy also offers Fable is unknown.
- Re-check `spec/agent.yaml` and recent commits before starting; 1203 may change name resolution.
