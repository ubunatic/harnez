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

## Architecture (a942, sol:med, 2026-10-25)

- Identity: harness + upstream provider + exact model/version + tier; aliases kept separate. Each fact is
  `{value, source, observed_at, evidence}` with source `spec-default`, `web`+URL, `discovered`+method/time/account,
  `measured`+evidence IDs, or `user-declared`; conflicting evidence is kept.
- Merge: catalog facts, then access annotation, then policy. Policy (`$XDG_CONFIG_HOME/harnez/models.yaml`):
  field-level patches, custom entries, version pins, `default_enabled: false` + explicit enabled IDs; arrays
  replace; explicit clearing. Enforced after alias resolution, incl. the Pi direct-ID path
  (`internal/subagent/driver.go:385`). Denied, unknown and stale access shown separately; unknown access does not
  disable subscription defaults.
- Costs: existing `cost` stays a relative estimate (`spec/agent.yaml:57`); new subscription facts
  (plan/currency/period/quota) and API rates (per-million input/output/cache, effective date). Unknown is null.
- Files: defaults in `spec/agent.yaml` + schema (`spec/schemas/agent.schema.json:91`); refreshed public facts,
  access, evidence and immutable snapshots in `$XDG_STATE_HOME/harnez/models/`. Nothing machine- or
  account-specific in git.
- Loading: one immutable merged catalog replaces `loadModelAliases/loadModelGuides`
  (`internal/subagent/agentspec.go:179`) and feeds `KnownModelEntries`, `ResolveModel` (`driver.go:201,245`),
  `agent models` (`cmd/harnez/agent_cmds.go:123`), completion and TUI. Quota "availability" stays separate.
- Discovery (explicit `update` only): parse `agy models`; claude `-p "/model"` for aliases (owner find, after
  the design was written), then bounded probe calls for claude/codex; API listing opt-in. Probe consent and
  spec limits; stop on quota/auth errors. Listing proves advertisement, a successful call proves access;
  ambiguous errors stay unknown; an incomplete run never removes a model.
- Measured capabilities: today telemetry `ToolCall` and `rate` carry no model capability; new capability
  observations tied to version/harness/tier and a test definition. Only attributable tests prove or disprove;
  contradictions become disputed; a new version resets them.
- Change report: diff of two snapshots; rows and JSON `{category, model, field, before, after, sources, evidence,
  severity}`; proposed vs. applied kept apart; defaults and pins never change silently.

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
