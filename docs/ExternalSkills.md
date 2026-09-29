# External Skills

`harnez skill` installs third-party agent skills (a directory with `SKILL.md`) from git
repositories. Code: `internal/skillreg`, `cmd/harnez/skill.go`. History: issue 631.

## Model

- Registry: `~/.harnez/skills/registry.yaml` plus pinned clones in `src/<name>@<sha12>`
  (`$HARNEZ_SKILLS_HOME` overrides). Clones are staged under that root, not `/tmp`, so the
  final rename never crosses filesystems.
- Only the skill directory is copied. Plugin hooks, MCP servers, commands, and agents are never
  installed; `explore` reports them. Nothing from the repository is executed.
- Every installed copy carries `.harnez-external`. harnez never overwrites or removes a skill dir
  without it, and `apply`/`diff`/`status`/`revert` never touch a dir with it.

## Explicit-Only by Default

Agents use an external skill only when the user names it; when several could fit, they list
matches and ask (rule "External Skills" in the generated `.harnez/rules/Tools.md`).

| Agent  | Copy | Switch (verified live 2026-09-29)                          |
|--------|------|------------------------------------------------------------|
| Claude | yes  | `disable-model-invocation: true`; `/name` still works      |
| Codex  | yes  | `agents/openai.yaml` `policy.allow_implicit_invocation: false` |
| Gemini, Prime | no | load via `harnez skill show <name>`                  |

`install --auto` installs plain copies everywhere.

## Names

Upstream names are kept, because skills reference each other by name. On conflict, `install
--as <prefix>-<name>` renames (frontmatter `name:`, stored as `upstream:`, kept by `update`) and
warns which files still mention the old name. harnez-managed skill names are reserved;
decommissioned names are not, because `apply` never removes a marked dir.

## Updating

Local edits to external skills are not kept: install upstream unmodified and put harnez deltas
in repo rules (issue 632). `update` compares the new commit with the cached one and reports:

- changed: diff summary (`--diff` for the full diff), then reinstalls;
- moved: same name at another path, followed automatically;
- gone: kept at the old commit, with rename candidates taken from lines in the repo's
  markdown (changelog, docs) that name the old skill next to a current one;
- skills the repo added since, listed but never installed.

A pinned ref stays pinned; `--latest` follows the default branch. `--dry-run` only reports.
Trial 2026-09-29 against mattpocock/skills (07-04 -> 09-18): three skills updated, `to-prd`
reported gone with `to-spec` as the top candidate.

## Pitfalls

- Skill scripts may resolve their workspace from the cwd (scroll-craft does); run them from the
  project, never from the install dir.
- Skills needing tools or keys ship their own check (e.g. `scripts/doctor.mjs`); `explore` names
  it but does not run it.
