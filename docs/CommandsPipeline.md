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

1. `ApplyAll` creates `~/.claude/commands/` and, when configured, `~/.prime/agent/prompts/`.
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

Each skill is a directory containing a `SKILL.md` file. harnez installs these to the
configured Gemini, Codex, Claude Code, and Prime Agent skill roots.

### Frontmatter difference

Skills use `name:` + `description:` (Antigravity format); Claude commands use `description:`
only. `genSkillContent` handles this — do not use `genCommandContent` for skills.

```yaml
skills_target: ~/.gemini/skills   # default; override to ~/.gemini/antigravity-cli/skills for agy-only
codex_skills_target: ~/.codex/skills
claude_skills_target: ~/.claude/skills  # real Agent Skills dir — description-matched, auto-loaded
prime_agent_target: ~/.prime/agent  # prompts/, skills/, AGENTS.md, and docs/

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
| Claude Code | `~/.claude/skills/<name>/SKILL.md` |
| Prime Agent | `~/.prime/agent/skills/<name>/SKILL.md` |
| Project | `<project>/.agents/skills/<name>/SKILL.md` |

`skills_target` controls the Gemini/Antigravity target. `codex_skills_target` controls the
Codex target. `claude_skills_target` controls Claude Code's own personal Agent Skills dir
(`~/.claude/skills/<name>/SKILL.md`, distinct from `~/.claude/commands/` — Skills are
description-matched and auto-loaded on relevance; commands stay manually invoked via `/name`).
`prime_agent_target` is the Prime Agent root; leave it empty to disable all Prime Agent
outputs. Prime prompt templates reuse command format unchanged because both formats accept
`description` frontmatter.

### Adding a new skill

1. Write or reuse `commands/<name>.md` as the body.
2. Add an entry to `config.yaml` → `skills:`.
3. Run `make apply`.
4. Verify: the `skills:` summary line and `make status` should show the name in every
   configured skill target.

Skills may declare supporting resources when their body refers to companion files:

```yaml
skills:
  - name: docup
    file: docs/commands/Docup.md
    resources:
      - source: docs/commands/DocupTesting.md
        target: references/DocupTesting.md
```

Sources are read from the embedded/config filesystem. Targets are copied below
the installed skill directory and are included in apply, diff, status, clean,
and idempotency checks. Keep resource targets relative to the skill directory;
the installer rejects absolute paths and parent traversal.

### Source reuse

A single `commands/<name>.md` file can back both a Claude command (`commands:` entry) and
an Antigravity, Codex, or Prime Agent skill (`skills:` entry). The frontmatter is generated
differently by `genCommandContent` vs `genSkillContent`; the body is identical.

---

## Model Configuration & Unmanaged Defaults

`config.yaml` supports `model:` to write an explicit model setting to `settings.json`.

**Convention**: Leave `model` omitted or commented out in `config.yaml` by default.
- Modern AI coding assistants rapidly advance model generations (e.g. Sonnet 3.5 → 3.7 → 4.6 → 5).
- Hardcoding aliases in repo config causes `harnez apply` to pin older model strings, overriding user or upstream default selections.
- When `model` is omitted from `config.yaml`, `harnez` leaves `settings.json`'s model property unmanaged.
