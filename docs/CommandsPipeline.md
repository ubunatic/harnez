# Commands & Skills Pipeline

How slash commands and agent skills flow from `config.yaml` to the target agent directories.

## Registration requirement

Dropping a `.md` file into `commands/` is not enough. The command must be declared in
`config.yaml` under the `commands:` list, or `ApplyAll` will never touch it.

Two forms:

```yaml
commands:
  # inline content
  - name: standup
    description: "Summarize today's git activity"
    content: |
      Run `git log --since=midnight --oneline` and summarize what was done.

  # file reference — content lives in commands/<name>.md in the repo
  - name: evergreen
    description: "Create/update evergreen docs and issue files for the current session"
    file: commands/evergreen.md
```

`file:` is resolved relative to the config dir via `fs.ReadFile` on `cfg.FS` (the embedded
or OS filesystem). It overrides `content` if both are set.

## Apply flow

1. `ApplyAll` creates `~/.claude/commands/` if absent.
2. For each entry: generate content → compare with existing file → write if changed.
3. Front-matter (`description:`) is prepended as YAML if `cmd.Description != ""`:
   ```
   ---
   description: "..."
   ---
   <body>
   ```
4. All command names are listed in the apply summary under `commands:` regardless of
   whether they were changed in this run (bug prior to 2026-06-23: only unchanged
   commands appeared — see issue #008).

## Discovery pitfall

`make status` shows `missing` for a command that is registered but not yet applied.
`make apply` installs it. The apply summary `commands:` line is the ground truth for
what is currently installed.

## Adding a new command

1. Write `commands/<name>.md` in the repo.
2. Add an entry to `config.yaml` → `commands:` with `file: commands/<name>.md`.
3. Run `make apply`.
4. Verify with `make status` — the name should appear in the `commands:` line.

---

## Agent Skills (`skills:`)

Each skill is a directory containing a `SKILL.md` file. claudeconfig installs these to
`~/.gemini/skills/<name>/SKILL.md` and, when configured, to
`~/.codex/skills/<name>/SKILL.md`.

### Frontmatter difference

Skills use `name:` + `description:` (Antigravity format); Claude commands use `description:`
only. `genSkillContent` handles this — do not use `genCommandContent` for skills.

```yaml
skills_target: ~/.gemini/skills   # default; override to ~/.gemini/antigravity-cli/skills for agy-only
codex_skills_target: ~/.codex/skills

skills:
  - name: evergreen
    description: "Create/update evergreen docs and issue files for the current session"
    file: commands/evergreen.md   # same source file as the Claude command
```

### Scopes (Antigravity)

| Scope | Path |
|-------|------|
| Shared (all agents) | `~/.gemini/skills/<name>/SKILL.md` |
| Antigravity CLI only | `~/.gemini/antigravity-cli/skills/<name>/SKILL.md` |
| Codex | `~/.codex/skills/<name>/SKILL.md` |
| Project | `<project>/.agents/skills/<name>/SKILL.md` |

`skills_target` controls the Gemini/Antigravity target. `codex_skills_target` controls the
Codex target. Leave a target empty to skip that install.

### Adding a new skill

1. Write or reuse `commands/<name>.md` as the body.
2. Add an entry to `config.yaml` → `skills:`.
3. Run `make apply`.
4. Verify: the `skills:` summary line and `make status` should show the name in every
   configured skill target.

### Source reuse

A single `commands/<name>.md` file can back both a Claude command (`commands:` entry) and
an Antigravity or Codex skill (`skills:` entry). The frontmatter is generated
differently by `genCommandContent` vs `genSkillContent`; the body is identical.
