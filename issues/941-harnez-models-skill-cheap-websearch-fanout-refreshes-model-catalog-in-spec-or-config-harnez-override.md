# 941 — harnez-models skill: cheap websearch fanout refreshes model catalog in spec or ~/.config/harnez override

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `spec/agent.yaml` (`models:` catalog), `internal/usage/localconfig.go` (`~/.config/harnez/local.yaml`), gokay skills (`../gokay/internal/skill/skills/`)

---

/goal A `harnez-models [update | ...]` skill runs a fanout of cheap web searchers and refreshes the model
catalog: in the harnez repo it updates `spec/agent.yaml`; elsewhere it writes a user override under
`~/.config/harnez/` that the installed binary loads over its built-in defaults, with tests for the merge; or stop
and report on a design choice below. Circuit breaker: on a loop or no progress, clear the goal and write a
failure report here.

## Problem

Model facts go stale between releases: new versions, changed prices, new capabilities, known pitfalls, new
models. Today they live only in `spec/agent.yaml` (`models:` with aliases, names, relative `cost`, `EFF`, skills
rating), so users get fresh data only by installing a newer binary.

## Owner request (2026-10-25)

- Skill like gokay's own skills (`gokay`, `gokay-sweep`: shipped by the binary, `SKILL.md` with an invocation
  section). Name `harnez-models`, first subcommand `update`; more arguments later.
- `update` starts cheap searchers in parallel (`--role advisor`, low tier), one angle each: version updates,
  cost changes, capabilities, pitfalls, new models. Findings cite sources (no link, no finding).
- Run inside the harnez repo: update the spec defaults. Run anywhere else: update a model config in
  `~/.config/harnez/`, which harnez tooling reads and which overrides its built-in defaults.

## Open decisions

1. Override file: a new `~/.config/harnez/models.yaml` vs. a `models:` key in the existing `local.yaml`.
   Recommendation: separate `models.yaml` with the same schema as the spec `models:` block.
2. Merge rule: per-model key override (field-level) vs. whole-entry replace. Recommendation: field-level,
   unknown keys rejected by the schema.
3. Where the skill source lives: `docs/commands/harnez-models.md` (installed by `harnez apply`) vs. embedded in
   the binary like gokay. Recommendation: follow the existing `docs/commands` path.
4. Who applies changes: the skill shows a diff and asks before writing, or writes and reports. Recommendation:
   show the diff, write on owner confirmation; model choices belong to the owner.

## Decision (owner, 2026-10-25)

All four recommendations accepted: separate `~/.config/harnez/models.yaml` with the spec `models:` schema;
field-level merge, unknown keys rejected; skill source in `docs/commands/harnez-models.md`; the skill shows the
diff and writes only on owner confirmation.

## Scope change (epic 942, owner, 2026-10-25)

941 is now child B of epic 942: layered loader and policy enforcement only. The web-refresh skill moved to 949;
formats come from 945. Two decisions above change: web refreshes write to `$XDG_STATE_HOME/harnez/models/`, not
into `models.yaml`; an invalid `models.yaml` stops harnez with a clear error instead of falling back to defaults
(fallback could re-enable denied models). One merged immutable catalog replaces `loadModelAliases/loadModelGuides`
(`internal/subagent/agentspec.go:179`) and feeds `KnownModelEntries`, `ResolveModel`, `agent models`, completion and
TUI; policy is enforced after alias resolution, incl. the Pi direct-ID path (`internal/subagent/driver.go:385`).
The acceptance below reads with these changes; the skill item belongs to 949.

## Decision (M1, 2026-10-25)

- Machine public catalog and access files are `$XDG_STATE_HOME/harnez/models/catalog.yaml` and
  `$XDG_STATE_HOME/harnez/models/access.yaml` (falling back to `~/.local/state/harnez/models/`);
  policy stays `$XDG_CONFIG_HOME/harnez/models.yaml` (falling back to `~/.config/harnez/models.yaml`).
- Legacy `spec/agent.yaml` model entries map `provider` to both `identity.harness` and
  `identity.provider`, and `name` to both `identity.model` and `identity.version`; `tier` is retained.
  These entries do not record upstream-provider or release-version facts, so the loader does not infer them.
  Legacy aliases and listing guidance remain separate from provenance facts; legacy `cost` remains guidance
  until consumers migrate.
- Missing machine and policy files are optional. Present files are schema-validated; a malformed or invalid
  file returns an error. Access observations are retained as evidence and unknown status leaves a model enabled.
- A policy patch stores field values as overrides (including explicit null clearing) alongside catalog facts;
  arrays replace. Full access enforcement is M2.

## M1 delivered (5b998705, sol:med): merged catalog loader

`internal/modelcatalog/catalog.go` loads spec defaults, state catalog/access and policy; invalid policy errors
without fallback; pins checked against inherited identity; XDG isolated in tests. Host: package tests green
with `-race`; developer `make test-q1` green (before the last package-only change).

## M2 — Pre-Work (host)

No refinements from M1. M2 per plan: replace `loadModelAliases/loadModelGuides`, enforce policy after alias
resolution incl. Pi direct IDs; tests for disable-all-but-N, enablement precedence, alias/direct-ID bypass,
pin mismatch, unchanged behaviour without policy.

## M2 delivered: merged-catalog consumers and policy gate

- Removed `loadModelAliases/loadModelGuides`; aliases, listing guidance, model entries, and resolution now use
  the cached merged catalog. `agent models` shows each row's source and filters disabled variants.
- Resolution enforces policy on the final provider/model/tier, including Pi direct IDs. Policy entries apply
  after alias resolution, inherited version pins are checked, contradictory enablement patches for aliases of
  the same identity fail with an error, and unknown access remains distinct from policy denial.
- Tests cover disabling all but three variants, explicit enablement precedence, shorthand and backend-ID
  bypasses, Pi direct-ID denial, invalid policy errors, and displayed source. CLI verification with isolated
  HOME/XDG directories listed exactly the enabled three; an invalid policy stopped `agent models`.
- `make test-q1` passed with no `--- FAIL` matches. The final alias-conflict validation was added afterward;
  `go test ./internal/modelcatalog ./internal/subagent ./cmd/harnez -count=1` passed on the final code.
  `make install` completed.

### M3 notes

- `agent models` currently shows a coarse source label (`spec-default`, `state`, or `user-declared`). It does
  not yet display fact-level evidence or detailed access observations.

## Decision (M3, 2026-10-25)

- Policy IDs keep a lowercase provider prefix and accept uppercase letters and `/` in the case-sensitive model
  portion. Runtime policy matching preserves model-ID case while comparing provider names and tiers without
  case sensitivity.

## M3 delivered: consumers and startup

- Agent-spec startup now loads the merged catalog first, so malformed policy stops agent operations with the
  catalog error. Model resolution, listing, completion, and TUI selection continue through the merged catalog
  and policy gate.
- Policy ID schema and both `enabled` and `models` keys accept exact Pi IDs with uppercase and `/`; fixtures
  cover valid and invalid IDs. Runtime tests prove an exact Pi ID is enabled and a case-changed ID is denied.
- `docs/other/Spec.md` and its root copy document the provider prefix, exact model-ID casing, allowed `/` and
  uppercase characters, and invalid whitespace.
- Host: developer `make test-q1` green on the final source.

## Decision (host, review r941, 2026-10-25; reversible)

- Resume/send of a session whose model the policy now disables: warn on stderr and continue (in-flight work is
  not cut off); new starts stay blocked.
- A disabled default model: the error names the model and says to pass `--model` or enable it in `models.yaml`.
- Access observations for an unknown identity are kept as evidence only; they never create catalog models.
- Policy `fields:` that the loader does not apply are rejected (schema and loader), not silently ignored.
- Typed facts are 947's job, a serialized catalog form is 948's; M4 only makes the exported catalog read-only
  (copies or accessors) and keeps per-observation `observed_at`.

## M4 — Pre-Work / Required Refinements (reviewer r941, opus:med)

1. Test isolation: `TestMain` (or shared helper) setting temp `HOME`/`XDG_CONFIG_HOME`/`XDG_STATE_HOME` in
   `internal/subagent`, `internal/tui/router`, `internal/bench` and any other package that loads the catalog.
2. One enablement rule: remove `Model.Enabled` or derive it from `IsEnabled` (`catalog.go:298-310` vs `:404`);
   fix tests at `catalog_test.go:24,87,93`.
3. Resume/send policy check per Decision (`agent_run.go:641`, `agent_interactive.go:190`, `agent_send.go:498`),
   with a test.
4. Disabled default model message per Decision (`agentspec.go:226`), with a test.
5. `catalog.go:237-238`: unknown-identity observations do not add models; test with status `denied`.
6. Reject unapplied policy fields (only `aliases`, `purpose`, `tier` are applied today), with an invalid fixture.
7. Assertions: `catalog_test.go:87` use an unlisted model with `default_enabled: true` and status
   `unknown`/`denied`; `:90` use `_, ok :=`; `:171` write a conflicting `$HOME/.config/harnez/models.yaml` and
   assert it is ignored; drop unused `HARNEZ_*` at `:153`; disable-all-but-N asserts which three and calls
   `KnownModelEntriesE`/`KnownModelSpecsE`.
8. Edge cases: `enabled:` list vs `models.X.enabled: false`; golden `KnownModelSpecs` unchanged without policy;
   tier-less vs tiered alias conflict (test + Spec.md); pin on a custom entry's own identity.
9. Read-only exported catalog and per-observation `observed_at` (Decision).

## M4 delivered

- Catalog state is private and exposed through detached-copy accessors; `Model.Enabled` was removed so
  `IsEnabled` is the only enablement rule. Access observations remain evidence even for unknown identities,
  and now carry required per-observation timestamps.
- Unapplied policy fields are rejected by both schema and loader. Added policy precedence, alias-conflict,
  custom identity pin, unknown/denied observation, XDG-over-HOME, disabled-default, and read-only API tests.
- Resume and send warn when current policy disables a stored session model, while continuing in-flight work;
  new start paths still resolve through the policy gate. Added resume/send warning coverage and a no-policy
  `KnownModelSpecsE` golden.
- Test mains isolate HOME, XDG and HARNEZ variables in model catalog, subagent, router and bench packages.
- Updated `docs/other/Spec.md` and its root copy for supported policy fields and access timestamps.
- `make install` succeeded. The single `make test-q1` attempt exited at its gofmt preflight before tests ran;
  `grep '--- FAIL'` had no matches. The test files were formatted afterward; no second q1 run was made.

## Acceptance

- Skill registered and installed; `update` dispatches the fanout and aggregates a cited table.
- Loader merges the override over embedded spec; tests: no file, partial override, invalid file (clear error,
  fall back to defaults), new model added.
- `harnez agent models` (or the current catalog view) shows overridden values and marks their source.
