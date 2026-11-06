# 1157 — Jules backlog batch: skills and sprint workflow (2 of 5) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 288, 313, 322, 333, 351, 366, 476, 493, 552

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 288 Add a `/harnez-agent` skill: lean fresh-handoff sprint that dispatches to an external agent CLI instead of a same-vendor subagent (covers `codex` and `agy`)
- 313 Canary convention: require go.work/GOWORK probe before trusting scratch-module dependency checks
- 322 Evolve /story skill with optional focus areas, tooling fit, and human-steering divergence analysis
- 333 Retire docs/feedback/, consolidate into docs/studies/ with labels/categories for grouping and filtering
- 351 Add AGENTS.md pointer: check config.yaml/Spec.md before hardcoding named business-value lists in Go
- 366 Automated doc compression command/skill with LLM canary evaluation loop
- 476 Improve agent start/resume UX with explicit sync, async, workdir, and planning controls
- 493 Add mixed subagent dispatch mode and make sprint skills dispatch-mode aware
- 552 Discovery: fast LLM classifier for exec command names (Needle or similar)

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1157.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1157 | 11850455728972557721 | pending | running (started 2026-11-06 01:23, audited 2cf3be4d) |
