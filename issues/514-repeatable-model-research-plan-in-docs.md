# 514 — Repeatable model research plan in docs/

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/Models.md` (2026-09-23 snapshots), `docs/practices/ModelRoles.md` ("Evaluating models"), `docs/ModelAdvisoryEval.md`, `spec/agent.yaml` (cost/eff/skills/roles/use), [[507-capture-subscription-quota-snapshots-at-harnez-agent-start-resume]], [[510-add-a-web-only-researcher-role-for-harnez-agent]], [[512-check-that-docs-list-only-model-aliases-defined-in-spec-agent-yaml]]

## Problem

On 2026-09-23 the `harnez agent models` guidance (COST × luna, EFF, SKILLS Go/TUI/SQL,
ROLES, USE) was built ad hoc from two web sweeps of 11 luna researchers, user cost
observations and past sprint evals. What we learned about that process lives only in
the chat:

- Researching one model per agent gave ratings that don't line up between models (e.g.
  conflicting sol/terra prices). One researcher per column (all models, one scale) fits
  COST and EFF better.
- Web evidence for Go, TUI and SQLite is missing for every model, so re-searching those
  columns only re-labels guesses; real evidence needs repo-local canaries with known answers.
- Model ids must be checked against the provider's own model list
  (`~/.codex/models_cache.json` slugs), not docs or research. `codex:astra` pointed at a
  nonexistent `gpt-5.6-astra`.
- Research prompts must say "web research only" and run outside the repo.
- Cost needs two anchors (luna = 1, astra = 100) plus user-stated ratios; list price ≠
  subscription cost.

## /goal

A `docs/ModelResearch.md` evergreen that an agent can follow unaided to refresh the model
guidance when a new model arrives or prices change, ending in an updated
`spec/agent.yaml`, `docs/Models.md` snapshot and a checked `harnez agent models` table.
It covers:

- **Triggers**: new model or id in a provider's model list, price change, a provider
  quota change, or repeated role failures seen in sprints.
- **Steps, cheapest first**: (1) id check against provider model lists; (2) column-wise
  web researchers (COST, EFF) on `luna:med`, web-only; (3) repo-local skill canaries
  (Go, TUI, SQLite with a fan-out trap) run on the candidate models, scoring accuracy
  and tokens per goal; (4) the repo-graded advisory eval from `ModelRoles.md` for role
  changes; (5) update spec values and the dated `Models.md` snapshot; (6) run the table
  in a pseudo-terminal, then commit.
- **Scope rule**: only the changed or new models are re-run, except when an anchor
  (luna, astra) changes.
- Prompt templates, expected token cost per step, and the cleanup of research sessions.

## Open questions

- Whether the skill canaries become a `harnez` command or a script under `scripts/`
  (possibly its own ticket).
- Whether a new slug in `models_cache.json` should trigger a reminder automatically.

## Done when

- `docs/ModelResearch.md` exists, is listed in `docs/README.md`, and is linked from
  `docs/Models.md` and `docs/practices/ModelRoles.md`.
- A dry run of steps 1 and 2 against the current lineup follows the doc without extra
  instructions.
