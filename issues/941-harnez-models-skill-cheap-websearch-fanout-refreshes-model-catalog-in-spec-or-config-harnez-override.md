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

## Acceptance

- Skill registered and installed; `update` dispatches the fanout and aggregates a cited table.
- Loader merges the override over embedded spec; tests: no file, partial override, invalid file (clear error,
  fall back to defaults), new model added.
- `harnez agent models` (or the current catalog view) shows overridden values and marks their source.
