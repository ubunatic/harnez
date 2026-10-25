# 942 — Epic: model catalog, access discovery and user model policy

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Epic
**Related**: 941, 943, `spec/agent.yaml` (`models:`)

---

/goal Harnez knows which models exist (public catalog with versions, capabilities, pitfalls, subscription vs.
API cost), which ones this user can actually call (discovered from the installed harness CLIs and vendor APIs),
and what the user allows (policy file that can disable, pin or add models, incl. custom ones); an update prints
a change report. Every task below is closed, skipped with a reason, or parked. Circuit breaker: on a loop, a hard
blocker or no progress, clear the goal and write a failure report here (tried, why it failed, resume point).

## Owner scenario (2026-10-25)

- Subscription users (this machine: Claude, Google Pro with Argon, OpenAI) always get the latest models; harnez
  defaults target them.
- Companies may only have older versions (e.g. Opus 4.x, Sonnet/Haiku 4.6), via API only, possibly on-prem, or a
  custom model without public data. The user must be able to replace the defaults completely, e.g. disable all
  but three models.
- Update has two parts: refresh public facts (web), and refresh access (add what became available, remove what
  is gone), by calling the installed CLIs and vendor APIs.
- The update ends with a change report: new or removed models and versions, cost up/down, capabilities gained,
  assumptions proven or disproven by continuous testing, wrong configuration.

## Design direction (owner chose path A, 2026-10-25)

Three layers, each fact carrying its source: public catalog (shipped spec, refreshed by the skill), access
(discovered per machine), user policy (`~/.config/harnez/models.yaml`). Sources: spec default, web finding with
link, discovered, measured (telemetry / `harnez rate`), user-declared. The change report diffs two snapshots.

## Out of scope

Automatic model selection changes without owner confirmation; billing integration.

## Open decisions

Left to the architect (sol:med) after the canary: snapshot storage, discovery cadence, how measured evidence
overrides an assumed capability, cost fields (subscription vs. API).

## Tasks

| Ticket | What | Group | Searcher → architect → developer | Reviewer | Status |
|---|---|---|---|---|---|
| 943 | Canary: what claude/codex/agy report about available models and access errors | 1 | probe advisor | host | Open |
| — | Architecture: layers, schema, sources, snapshot diff | 2 | — → sol:med → — | host + owner | Open |
| 941 | Layered loader + `models.yaml` policy (adjust to the design) | 3 | tbd | tbd | Open |
| — | Access discovery per harness CLI | 3 | tbd | tbd | Open |
| — | `harnez-models update` web-refresh skill (part of 941) | 4 | tbd | tbd | Open |
| — | Measured capabilities from telemetry / rate data | 4 | tbd | tbd | Open |
| — | Change report | 5 | tbd | tbd | Open |
| — | sprint study, `/evergreen` | final | host | — | Open |
| — | `/wrap` | final | host | — | Open |
