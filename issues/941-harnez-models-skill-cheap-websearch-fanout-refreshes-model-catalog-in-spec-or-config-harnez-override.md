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

## Acceptance

- Skill registered and installed; `update` dispatches the fanout and aggregates a cited table.
- Loader merges the override over embedded spec; tests: no file, partial override, invalid file (clear error,
  fall back to defaults), new model added.
- `harnez agent models` (or the current catalog view) shows overridden values and marks their source.
