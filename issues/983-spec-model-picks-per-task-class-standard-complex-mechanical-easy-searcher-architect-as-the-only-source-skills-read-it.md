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

## Architecture (arch-983, opus:low, 2026-10-27)

- **Spec:** top-level `purpose_aliases` in `spec/agent.yaml` (map alias → ordered list), plus `purpose_roles` and
  `purpose_traits` (one-line meaning each, used by help and generated docs). Schema property, `agentSpec` field,
  validation in `parseAgentSpec`.
- **Names:** `^(dev|search|review|arch):[a-z][a-z0-9-]*$`; role is a closed enum; trait must not be low/med/high or
  any tier synonym (Go check with the resolver's synonym table). Value: ordered list, no duplicates; `[]` = unset,
  resolving it is an error. Each entry resolves via `resolveModelIn` or is another purpose alias; spec load fails on an
  unknown target.
- **Resolution:** `resolvePurpose` in `resolveModelWithCatalog` (`driver.go:400`) before `resolveModelIn`, only when
  the part before the first `:` is a role. Role names are reserved: spec load fails if a provider, model alias or role
  alias is named dev/search/review/arch (test). Depth-first expansion with a visited set, depth limit 4, checked at
  load and at runtime.
- **Fallback availability:** enablement (`IsEnabled`) from M1; quota from M3 via the availability function used by
  `RejectExhaustedQuota` (`agent_run.go:205`), skipped unless `--allow-exhausted-quota`; access observations: only
  `denied` skips, `unknown` counts as available. Fallback taken → one stderr line
  `dev:smart → opus:med (sol:med: quota exhausted)`; all fail → non-zero exit with one reason per entry and a hint to
  pass `--model provider:model:tier`.
- **User override:** `purpose_aliases:` in `~/.config/harnez/models.yaml` (`mergePolicyFile`, `catalog.go:378`,
  `model-policy.schema.json`); a user entry replaces that alias's whole list; users may add traits, not roles.
  Precedence: concrete `--model` > user override > embedded spec; then enablement and quota.
- **Visibility:** session record `ModelAlias string json:"model_alias,omitempty"` (`session.go:16`); `agent models`
  second table (alias, chain, resolved now, skip reason) incl. `--json`; `agent list` model column
  (`dev:smart→sol:med`); start banner `model: opus:med via dev:smart (fallback 2/2)`.
- **Prose:** skills name aliases only. `harnez agent models --aliases --md` prints the table from `purpose_roles` /
  `purpose_traits`; docs carry a generated block `<!-- harnez:purpose-aliases:begin/end -->` filled by `harnez init`
  with a staleness test; `TestNoModelPicksInPractice` greps `docs/commands` and `docs/practices` (full and lite) for
  `\b(luna|sol|terra|opus|haiku|argon):(low|med|high)\b` outside generated blocks, small allow-list for model docs.

## Host decisions (2026-10-27)

- The section 3.4 check named `sol:low` for `dev:smart`; the owner table says `sol:med`. Table wins; 3.4 fixed.
- `dev:gui` order lean, then std (host reading of "lean/std"), as recorded in the Decision table. Reversible.
- `dev:cheap` (`luna:med`) is the alias for mechanical work.
- Access gating: only `denied` skips a fallback entry (architect recommendation).
- `arch:` aliases: none until the owner picks; the role stays in the enum.
- Developer: `terra:med` (the owner's `dev:std`). Reviewer: `opus:low` diff review before close.

## Milestones

- **M1 (`dev:smart` resolves):** schema key, loader, name/value validation, reserved-role check, resolution with
  cycle/depth guard, Decision-table aliases in the spec. Visible: `harnez agent start --model dev:smart` resolves to
  `sol:med` (host runs it). Tests: resolution table, cycle `a→b→a`, depth 5, trait `low`, unknown target, provider
  named `dev`, unset `dev:ai` error, existing `provider:model:tier` and bare aliases unchanged. Pre-work: confirm
  `resolvePiModel` (first lookup in `resolveModelIn`) never claims `dev:smart`.
  - **M1 delivered (`f1b125b3`, `dev:smart` resolves):** spec keys, schema, `validatePurpose`, `resolvePurpose`.
    Developer `make test-q1`: 58 `ok`, 0 `--- FAIL`. Host live: `--model dev:smart` started `codex:gpt-6.1-sol`
    tier `med`; `dev:nonexistent` → `unknown purpose alias`; `dev:ai` → `purpose alias "dev:ai" is unset`.
- **M2 (`agent models` lists aliases):** alias table and JSON; golden test.
  - **Pre-Work / Required Refinements (from M1 review):**
    1. The role set is hard-coded twice in Go: the `reserved` map in `validatePurpose` and the `purposeName` regex
       (`agentspec.go`). Derive both from the keys of `purpose_roles` (the schema enum stays the closed set), so
       the spec is the only source (docs/Spec.md).
    2. `TestPurposeValidation` only asserts `err != nil`, so a case can pass for the wrong reason (e.g. a missing
       trait description instead of the rule under test). Assert an expected message substring per case.
    3. Add a runtime test for an unknown purpose alias (`dev:nonexistent` → `unknown purpose alias`).
- **M3 (fallback by enablement and quota):** availability function into the resolver, stderr fallback line, all-failed
  error. Tests with fake catalog/availability: first disabled, quota exhausted, all exhausted, override flag. Keep
  `resolveDefaultModelWithCatalog`'s "default model is disabled" message when the default is an alias.
- **M4 (user override in models.yaml):** tests: list replaced, cycle via override, unknown model, new role rejected.
- **M5 (alias visible):** session record field, `agent list` model column, banner. Tests: round-trip; old records load.
- **M6 (prose migration):** generated block, `--md`, the 7 docs (full and lite) use aliases, guard test; root copies
  via `harnez init -d .`; the 3.4 `rg` check empty.

## 3. Implementation & Verification Plan

1. Design: spec shape for purpose aliases (2b), the trait list per role, and the resolver order against model
   aliases. Per the epic rule, the schema ships with its first reader and a visible result
   (`harnez agent start --model dev:smart` resolves; `harnez agent models` lists the purpose aliases).
2. Schema entry, loader and validation (unknown model or class fails), `harnez agent models` shows the picks.
3. Replace the model names in the skills and practice docs listed in section 1 with the class reference; keep
   full and lite sources and root copies in sync (`harnez init -d .`).
4. Verify: tests for resolution and validation; `harnez agent start --model dev:smart` resolves to `sol:med`;
   `rg 'luna:med|sol:low|haiku:high' docs/commands docs/practices` shows no task-class picks left in prose;
   `make test-q1`; `make install`.
