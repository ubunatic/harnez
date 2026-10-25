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

## Decision (owner, 2026-10-25)

All five accepted: (1) machine data in `$XDG_STATE_HOME/harnez/models/`, policy in `models.yaml`, nothing of it
in git; (2) invalid `models.yaml` stops harnez; (3) unknown access stays enabled, an incomplete run never
removes a model; (4) `cost` stays relative, real subscription and API prices separate, unknown is null;
(5) probes ask first, stop on quota/auth errors, about one tiny call per model, cap about 20, limits in spec.
Added: probe calls run bare, with no harnez preface and minimal system prompt and context (see 946).

## Out of scope

Automatic model selection changes without owner confirmation; billing integration.

## Open decisions

Left to the architect (sol:med) after the canary: snapshot storage, discovery cadence, how measured evidence
overrides an assumed capability, cost fields (subscription vs. API).

## Tasks

| Ticket | What | Group | Searcher → architect → developer | Reviewer | Status |
|---|---|---|---|---|---|
| 943 | Canary: what claude/codex/agy report about available models and access errors | 1 | luna:low advisor | host | Closed (65511a42) |
| — | Architecture: layers, schema, sources, snapshot diff | 2 | — → sol:med → — | host + owner | Done (cdfb4b71) |
| 945 | A: formats and schemas | 3 | — → — → sol:low (M1-M2), sol:med (M3) | opus:low | Closed |
| 941 | B: merged loader + policy enforcement (needs 945) | 4 | — → plan → sol:med | opus:med | Closed |
| 946 | C1: one bare probe for one model, prints result + input tokens | 5 | — → plan → sol:low | host | Open |
| 954 | C2: `models discover` (listings) → access.yaml, ACCESS column in `agent models` | 6 | — → plan → haiku:high | host | Open |
| 955 | C3: bounded batch probes (confirm, budget, stop on quota/auth) | 7 | — → plan → haiku:high | sol:low | Open |
| 948 | F1: snapshot + text change report (added/removed/access); JSON and other categories later | 8 | — → plan → haiku:high | host | Open |
| 949 | D: `harnez-models update` web-refresh skill (needs 948 F1) | 9 | — → plan → sol:low | host | Open |
| 947 | E: capability evidence | — | — | — | Backlog: no attributable test source yet (its own exit clause) |

## Re-cut (host + owner, 2026-10-25)

945/941 took 3 and 6 review rounds: schemas were designed before any consumer and 941 bundled loader, override
semantics, migration and enforcement on every entry point. The rest runs as thin slices, each with one visible
result, a plan-first cheap developer and one review. 945's snapshot and change-report schemas are provisional:
948 may change them freely.
| — | sprint study, `/evergreen` | final | host | — | Open |
| — | `/wrap` | final | host | — | Open |
