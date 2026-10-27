# 983 — spec: model picks per task class (standard, complex, mechanical, easy, searcher, architect) as the only source; skills read it

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: 942 (epic: model catalog and user model policy), 974 (Terra onboarded), 973 (luna:med default, Haiku tiers), 893 (Haiku as searcher)

---

/goal Which model to use for which kind of task is defined once in `spec/agent.yaml` (with schema), harnez prints and
resolves it, and the sprint skills and practice docs refer to it instead of naming models in prose; or stop and report
when blocked on an owner decision or a denied permission. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Owner, 2026-10-27: "I want that everything is well-specced. Eventually, we need to get rid of the prose, and the model
configuration should be the only source of truth." Terra (974) should be the standard developer and Sol the model for
complicated tasks.

Today the spec has per-model `roles` (developer, advisor, reviewer, mechanical; a recommendation, not enforced) and
free-text `use` / `use_med` lines, but no task-class picks. The picks live as prose, repeated and drifting, in
`docs/commands/lean-sprint.md`, `lean-sprint-v1.md`, `epic.md`, `HarnezStatus.md`, `docs/practices/AgenticLoop.md`
(full and lite), `docs/practices/ModelRoles.md`, and the `use_med` line of `codex:luna` ("cheap default developer").
Every model change (973, 974) needed edits in all of them.

## 2. Proposed picks (owner direction plus current guidance)

| Task class | Model | Source |
|---|---|---|
| standard developer | `terra:low` | owner, 2026-10-27 |
| complex developer (judgment, orchestration, hard compiler failures) | `sol:low` | owner, 2026-10-27 |
| mechanical / well-specified cheap work | `luna:med` | host reading; owner to confirm |
| easy task | `haiku:med` | 973 |
| very simple, repetitive task | `haiku:low` | 973 |
| searcher (read-only discovery, web) | `luna:low`, `haiku:low` | lean-sprint §1, 893 |
| architect | `opus:low`, `opus:med`, `sol:low`, `sol:med`, `argon:low` | lean-sprint §1.5 |
| reviewer | open | owner decision |

## 2b. Owner direction: purpose aliases (2026-10-27)

Skills reference models through purpose aliases of the form `<role>:<trait>`, for example `dev:cheap`, `dev:fast`,
`dev:smart`, `dev:tui`, `dev:gui`, `dev:ai`, `dev:lean`. Alias names never use the effort words low/med/high; the
effort tier is part of what the alias resolves to. Nothing like this exists today: the spec has only per-model
aliases (`luna`, `sol:low`, `terra:med`) and role-name aliases (`coder`, `critic`), checked 2026-10-27.

Constraints for the design:
- `dev:` must not collide with the `provider:model:tier` syntax (`codex:terra:low`); `dev`, `review`, `search`,
  `arch` are not provider names, but the resolver must check that, and a future provider must not shadow them.
- Each alias resolves to one `provider:model:tier` from the spec; users can override it in their own config
  (941's user model policy, 925's spec overrides).
- Traits without evidence today (`tui`, `gui`, `ai`) need an owner pick or stay unset; `skills` ratings in the spec
  are mostly `?`.

## Decision (owner, 2026-10-27)

Initial purpose aliases. A value is an ordered list: the first available entry wins, later ones are fallbacks
(e.g. another vendor when one has no quota). A value may name another purpose alias.

| Alias | Resolves to | Note |
|---|---|---|
| `dev:lean` | `terra:low` | owner: the standard, a bit more efficient |
| `dev:std` | `terra:med`, `opus:low` | owner |
| `dev:smart` | `sol:med`, `opus:med` | owner |
| `dev:tui` | `dev:smart` | owner |
| `dev:gui` | `dev:lean`, `dev:std` | owner wrote "lean/std"; host reads it as lean first |
| `dev:cheap` | `luna:med` | host default from 973 ("etc.") |
| `dev:fast` | `haiku:med` | host default from 973 |
| `dev:ai` | unset | no evidence yet; owner pick later |
| `search:cheap` | `luna:low`, `haiku:low` | host default from lean-sprint §1 |
| `review:smart` | `sol:low` | host default; owner may change |

Design consequences: lists need a defined "available" test (spec availability, access, quota), alias-to-alias
references need cycle and depth checks, and the resolved model plus the alias used should show in the session
record and `harnez agent list`.

## Searcher findings (2026-10-27, luna:low, search-983)

- Resolution: `ResolveModel` (`internal/subagent/driver.go:393`) → `resolveModelWithCatalog` → `resolveModelIn`
  (provider:model[:tier], tier synonyms, bare aliases, `preferred` breaks bare-alias ties, `driver.go:452`, `:517`),
  then user enablement. Called from `cmd/harnez/agent_run.go:327`. Today `dev:smart` fails with
  `invalid model "dev:smart": expected provider:model[:tier]` (`driver.go:526`). Providers are only codex, claude,
  agy; no provider or alias named dev, review, search or arch.
- Catalog: `internal/modelcatalog/catalog.go:255` merges embedded spec, state catalog, access observations and
  `~/.config/harnez/models.yaml` (user patches and custom entries, `:378`). Access (accessible/denied/unknown) is
  stored but does not gate resolution (`:96`, `:545`). Enablement: `IsEnabled` (`:543`). Quota is a separate cached
  check at start (`agent_run.go:197`, `:205`); the resolver sees enablement only.
- Display: `agent models` table `cmd/harnez/agent_cmds.go:121`; session record fields provider/model/tier
  (`internal/subagent/session.go:16`); `agent list` omits model and tier (`cmd/harnez/agent_manage_cmds.go:119`).
- Schema: `models` at `spec/schemas/agent.schema.json:174`, top-level additional properties disallowed (`:240`);
  new key needs schema property, `agentSpec` field and validation in `parseAgentSpec` (`agentspec.go:148`, `:220`);
  schema helper `catalog.go:704`.
- Prose naming model tiers: lean-sprint.md 14, lean-sprint-v1.md 5, epic.md 6, HarnezStatus.md 2, AgenticLoop.md 36,
  AgenticLoop.lite.md 11, ModelRoles.md 10 lines.
- Searcher recommendation: `purpose_aliases` key, resolved before `resolveModelIn`, then enablement and quota per
  candidate, cycle/depth guard; first slice `dev:smart` resolves and `agent models` lists aliases, fallback next.

## 3. Implementation & Verification Plan

1. Design: spec shape for purpose aliases (2b), the trait list per role, and the resolver order against model
   aliases. Per the epic rule, the schema ships with its first reader and a visible result
   (`harnez agent start --model dev:smart` resolves; `harnez agent models` lists the purpose aliases).
2. Schema entry, loader and validation (unknown model or class fails), `harnez agent models` shows the picks.
3. Replace the model names in the skills and practice docs listed in section 1 with the class reference; keep
   full and lite sources and root copies in sync (`harnez init -d .`).
4. Verify: tests for resolution and validation; `harnez agent start --model dev:smart` resolves to `sol:low`;
   `rg 'luna:med|sol:low|haiku:high' docs/commands docs/practices` shows no task-class picks left in prose;
   `make test-q1`; `make install`.
