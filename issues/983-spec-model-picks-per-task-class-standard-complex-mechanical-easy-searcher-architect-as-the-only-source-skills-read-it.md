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

## Redesign review (argon-983, argon:low, 2026-10-27)

Owner, after M1: "We want to select a model and a role, and now we are mixing it." M2 was stopped mid-turn
(uncommitted edits in the tree). Argon confirmed four conflicts in M1's `<role>:<trait>` model names:
`--role reviewer --model dev:smart` gives a reviewer a developer pick; `--model search:cheap` without `--role` runs
as the default role, developer; `purpose_roles` (dev/search/review/arch) is a second role taxonomy beside
`roles:` (developer/reviewer/advisor); the reserved-name check forbids `dev`/`review` as natural role aliases.

Recommended design B: picks scoped per role,
`roles.<role>.models: {<trait>: [ordered list]}` plus `roles.<role>.default_model: <trait>`, a top-level trait
description map, and `dev`/`review`/`search`/`arch` as `roles.<role>.aliases`. Skills write
`harnez agent start --role dev --model smart`; `--role review` alone uses the reviewer default. Rejected A (global
traits): `smart`/`cheap` differ per role. About 75% of M1 survives (trait validation, tier-word ban, duplicate,
cycle/depth walk, test structure); `purpose_roles`, the `<role>:` prefix and the reserved checks go. 984 becomes
"guess the trait within the resolved role".

## Decision: design B (owner, 2026-10-27, "go B")

Replaces 2b's `<role>:<trait>` names and the architecture's `purpose_*` keys. Same picks, scoped per role:

```yaml
model_traits: {lean: ..., std: ..., smart: ..., tui: ..., gui: ..., cheap: ..., fast: ..., ai: ...}
roles:
  developer:
    aliases: [dev, ...]          # added to the existing role aliases
    default_model: lean
    models: {lean: [terra:low], std: [terra:med, opus:low], smart: [sol:med, opus:med], tui: [smart],
             gui: [lean, std], cheap: [luna:med], fast: [haiku:med], ai: []}
  reviewer:
    aliases: [review, ...]
    default_model: smart
    models: {smart: [sol:low]}
  advisor:
    aliases: [search, arch, ...]
    default_model: cheap
    models: {cheap: [luna:low, haiku:low], smart: [opus:low, opus:med, sol:low]}
```

- Resolution order: `--role` first (existing `resolveRoleIn`, aliases, decider), then `--model` within that role.
  A trait name looks up `roles.<role>.models`; anything else goes through today's resolver unchanged
  (`provider:model:tier`, bare model aliases). No `--role` means the default role, developer.
- A list entry is a model reference or another trait of the same role; cycle and depth checks as in M1.
- Trait names: listed in `model_traits` with a description, never a tier word or synonym, never equal to a provider,
  model alias or role name (spec load fails).
- Host decision: `default_model` applies when `--model` is omitted, replacing the global default for that role;
  a role without `default_model` keeps today's default. Reversible.
- Host decision: the advisor `smart` list (`opus:low, opus:med, sol:low`) is the lean-sprint §1.5 architect set;
  no separate `arch` trait.
- The stopped M2 working-tree edits (`agentspec.go`, `purpose_test.go`, `cmd/harnez/agent_purpose_test.go`) are the
  developer's to adapt or discard in M2.

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
Re-cut after the design-B decision (M1 shipped `<role>:<trait>`; M2 moves it into the roles):

- **M2 (role-scoped picks, design B):** move the M1 picks into `roles.<role>.models` / `default_model`, rename
  `purpose_traits` to `model_traits`, drop `purpose_roles`, `purpose_aliases`, the `<role>:` regex and the reserved
  checks; add `dev`, `review`, `search`, `arch` as role aliases; resolve the role before the model
  (`cmd/harnez/agent_run.go`). Visible (host runs): `--role dev --model smart` → `sol:med`; `--role review --model
  smart` → `sol:low`; `--role review` alone → `sol:low`; `--model smart` alone → developer `sol:med`;
  `--role review --model lean` → error listing the reviewer's traits. Tests: that table, trait cycle and depth 5,
  trait `low`, trait named like a model alias (`sol`) or provider, unset `ai` error, `default_model` naming a missing
  trait, concrete `provider:model:tier` and bare aliases unchanged.
  - **Pre-Work (carried from M1 review):** each validation case asserts an expected message substring, not only
    `err != nil`; a runtime test for an unknown trait. The old item 1 (derive the reserved set) is moot under B.
  - **M2 verified (role-scoped picks):** host `make test-q1`: 58 `ok`, 0 `--- FAIL`. Live: `--role review --model
    lean` → `unknown trait or model "lean" for role "reviewer"; known traits: smart`; `--role dev --model smart` →
    sol `med`, developer; `--role review --model smart` → sol `low`; `--role review` → `model: codex:sol:low
    (default)`. `--model smart` without `--role` ran as reviewer, sol `low`: the role was inferred from the prompt
    ("Reply with exactly: ok. Do not run any command.", 686 prompt inference), so role-before-model held; the
    ticket's "developer" expectation assumed no inference.
- **M3 (`agent models` lists the picks):** per-role table (role, trait, chain, resolved now) and JSON; golden test.
  - **Pre-Work (from M2 review):** `ResolveModelForRole` resolves the role again after `startRole` already did;
    pass the canonical role through without a second `resolveRoleIn` call, or document why both are needed.
  - **Host decision (M3 plan review):** `agent models --json` stays the unchanged model array; mixing pick records
    into it would break its consumers. Picks get `agent models --picks` (table only) and `--picks --json` (array of
    `{role, trait, chain, resolved, error}`); the plain table output also prints the picks table below the models.
    Reversible.
  - **M3 delivered (`c81c8fa8`, picks listed):** developer `make test-q1`: 58 `ok`, 0 `--- FAIL`. Host live:
    `agent models --picks` shows 11 rows, each resolved to its chain's first entry (`tui` → `codex:sol:med`, `gui` →
    `codex:terra:low`, `ai` → `unset`); `--json` model array unchanged; picks table under the plain listing.
    Canonical-role path added (M2 pre-work).
- **M4 (fallback by enablement and quota):** availability function into the resolver, stderr fallback line, all-failed
  error. Tests with fake catalog/availability: first disabled, quota exhausted, all exhausted, override flag. Keep
  `resolveDefaultModelWithCatalog`'s "default model is disabled" message when the default is a trait.
  - **Pre-Work (from M2 review):** `ResolveModelForRole` returns a plain `ResolveModel` error when a role's
    `default_model` points at a disabled model; it bypasses `resolveDefaultModelWithCatalog`. Test that case.
  - **M4 delivered (`3ff833f8`, fallback):** developer `make test-q1`: 58 `ok`, 0 `--- FAIL`. Tests cover each skip
    reason (disabled, denied, quota), nested `gui` → `lean` → `smart` order, unset, cycle, override bypassing quota
    only, default-disabled wording, latest access observation, and the CLI notice
    `smart → opus:med (sol:med: quota exhausted)`. Picks view reads cached quota (`CachedProviderQuotaAvailability`)
    and stored access only. Host live: picks view and unknown-trait error unchanged; a live fallback was not forced
    (would need a models.yaml edit or exhausted quota).
- **M5 (user override in models.yaml):** `roles.<role>.models` entries replace that trait's list; users may add
  traits, not roles. Tests: list replaced, cycle via override, unknown model, unknown role rejected.
  - **Pre-Work (from M4 review):** the all-failed test asserts only the reasons; also assert the
    `--model provider:model:tier` hint. Add one CLI test where a user override in a temp `models.yaml` disables the
    first entry, so the fallback notice is exercised end to end (host can then run it live with a temp
    `XDG_CONFIG_HOME`).
- **M6 (pick visible):** session record `model_alias` (e.g. `smart`), `agent list` model column, banner. Tests:
  round-trip; old records load.
  - **Pre-Work (from M3 review):** the picks table does not mark each role's `default_model`; add a marker
    (e.g. `smart*` or a DEFAULT column) and the matching JSON field.
- **M7 (prose migration):** generated block, `--md`, the 7 docs (full and lite) write `--role X --model <trait>`,
  guard test; root copies via `harnez init -d .`; the 3.4 `rg` check empty.

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
