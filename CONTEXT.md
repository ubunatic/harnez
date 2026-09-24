# harnez — project context

## What it is

`harnez` is a Go CLI tool that manages Claude Code and coding agent configuration declaratively from a single `config.yaml`. One source of truth drives all outputs: `settings.json`, `CLAUDE.md`/`AGENTS.md`, custom slash commands, skills (`~/.gemini/skills/`, `~/.codex/skills/`, `~/.claude/skills/`), and language doc copies. Apply is idempotent; user-managed keys in JSON files are preserved on every run.

## Architecture

| File | Responsibility |
|---|---|
| `main.go` | Cobra command wiring; `apply`, `init`, `diff`, `status`, `revert`, `clean` subcommands |
| `config.go` | YAML structs; `loadConfig()` sets `cfg.Dir` and `cfg.FS`; `loadConfigEmbedded()` uses `//go:embed` |
| `apply.go` | All generators, JSON helpers, MD block logic, and orchestrators |
| `status.go` | `runStatus`, `hasSettingsKey`, `hasSectionMD` |

`config.go` embeds `config.yaml` and the `commands/` directory at compile time (`//go:embed config.yaml commands`). When invoked without `-c`, the binary runs on its own embedded config. `cfg.FS` is either the OS filesystem (for `-c` path) or the embedded FS — command `file:` references read from whichever is active.

Source command files live in `commands/` (e.g. `commands/domain-modeling.md`) and are referenced via `file:` in `config.yaml`.

## Commands

| Command | What it does |
|---|---|
| `apply` | Global sync: merges managed keys into `settings.json`; writes CLAUDE.md sections, command files, skills, lang docs. No project-local work. |
| `init` | Project setup: creates AGENTS.md + CLAUDE.md symlink, applies config local sections, copies lang docs locally, scaffolds/injects Makefile targets. Defaults to cwd (`-d .`). |
| `diff` | Shows what `apply` would change, without writing. Uses `diff -u` on temp files. |
| `revert --managed` | Removes managed keys from `settings.json`; strips MD sections. |
| `clean [procs] [q1]` | Inspects or removes stale process groups and verified incomplete quota-1 state. |
| `status` | Prints config summary and checks which managed items are present on disk. |

All subcommands accept `-c <config>` (default: embedded) and `-t <target>` (default: `~/.claude`).

`make apply/diff/revert-managed/status` build first then run with the repo `config.yaml` and `~/.claude` target.

## Generated files

| File | Strategy |
|---|---|
| `~/.claude/settings.json` | JSON key merge: read existing → merge managed keys → validate → write. User keys preserved. |
| `~/.claude/CLAUDE.md` | HTML-comment managed blocks, one per `agents_md.global.sections` entry |
| `AGENTS.md` | HTML-comment managed blocks, one per `agents_md.local.sections` entry |
| `~/.claude/commands/<name>.md` | Whole file. Frontmatter `description:` from config; body from inline `content:` or `file:` |
| `~/.gemini/skills/<name>/SKILL.md` | Skill definition directory + markdown file for Gemini / Antigravity |
| `~/.codex/skills/<name>/SKILL.md` | Skill definition directory + markdown file for Codex |
| `~/.claude/skills/<name>/SKILL.md` | Real Claude Code Agent Skill — description-matched, auto-loaded on relevance (distinct from the always-present `~/.claude/commands/<name>.md` slash command) |
| `~/.claude/docs/<lang>.md` | File copy from `source` (relative to config dir). |

## Managed block format

Markdown files use HTML comment markers:
```
<!-- harnez:begin Section Name -->
...content...
<!-- harnez:end Section Name -->
```
`applySectionMD` / `diffSectionMD` / `cleanSectionMD` in `apply.go` handle find-replace-or-append. Backward compatibility supports legacy `<!-- claudeconfig:begin ... -->` markers transparently.

JSON files use **no markers** — keys are merged directly.

## settings.json merge strategy

`readJSONC` strips `//` line comments (handles legacy marker-style files) then uses `json.NewDecoder.Decode` — not `json.Unmarshal` — so trailing content after the first JSON value is silently ignored rather than causing a parse failure.

`applySettingsJSON`: read → merge our keys in → `json.Valid` check → write.

`cleanSettingsJSON`: delete all `managedSettingsKeys` from the parsed map, write back (or remove file if empty).

Managed keys: `model`, `effortLevel`, `permissions`, `hooks`, `env`, `spinnerVerbs`, `mcpServers`.

## Round-trip stability

All nested structures in `buildSettingsDoc` use `map[string]any` (not typed structs). `json.Marshal` sorts map keys alphabetically; `json.Unmarshal` also produces maps. This makes the read→marshal round-trip stable, so `diff` reports no changes after `apply`.

Typed structs would marshal in field declaration order, creating a permanent diff against the alphabetically-parsed existing file.

## Key design decisions

**JSON merge over managed blocks** — Claude Code's `settings.json` is a live file that Claude Code itself writes (theme, plugins, etc.). Comment-marker blocks appended after the closing `}` are invalid JSONC and get ignored or cause parse errors. Merging only owned keys preserves user/Claude Code keys.

**`json.Decoder` not `json.Unmarshal`** — The old approach left `// claudeconfig:begin` markers after the JSON object. `json.Unmarshal` fails on trailing content and returns an empty map; `json.Decoder.Decode` reads only the first JSON value and stops, recovering the object cleanly.

**`AGENTS.md` canonical, `CLAUDE.md` symlink** — project instructions live in `AGENTS.md` (works with any agent); `CLAUDE.md → AGENTS.md` lets Claude Code find it. Same convention globally: `~/.claude/CLAUDE.md` is real, `~/AGENTS.md` symlinks to it.

**`file:` for command bodies** — Inline `content:` in YAML is awkward for multi-line prompts. Commands support `file: commands/foo.md` to read the body from a repo file; `description:` still comes from config. The body is read via `cfg.FS`, so it works from the embedded FS when no `-c` flag is given.

**Embedded binary** — `config.yaml` and `commands/` are embedded at compile time. The binary is self-contained: running `harnez apply` (no `-c`) applies the built-in config. This lets the tool install itself idempotently after `make install`.

## Gotchas and learnings

**PATH shadowing** — The old Makefile installed to `~/.local/bin`; the new one installs to `/usr/local/bin` via `sudo install`. If `~/.local/bin` appears earlier in `$PATH`, the stale binary shadows the new one. Remove with `rm ~/.local/bin/harnez`.

**Claude Code overwrites settings.json** — Claude Code monitors `~/.claude/settings.json` and may restore a cached version immediately after a write during an active session. Apply from outside a running Claude Code session for settings changes to persist, or restart Claude Code after applying.

**Model alias resolution** — `model: sonnet` in config resolves to `claude-sonnet-4-6` before writing. Unrecognised values pass through unchanged. Current aliases: `sonnet→claude-sonnet-4-6`, `opus→claude-opus-4-7`, `haiku→claude-haiku-4-5-20251001`.

**`cfg.Dir` for relative paths** — `loadConfig` sets `cfg.Dir = filepath.Dir(configPath)`. Language `source:` paths and command `file:` paths are resolved relative to this, not to cwd. Running `harnez apply` from a different directory works as long as `-c` points to the config file.

## Symlink convention

- Global: `~/AGENTS.md` → `~/.claude/CLAUDE.md`
- Local: `./CLAUDE.md` → `./AGENTS.md`

`ensureSymlink(linkPath, target)` is a no-op if the link already points correctly; otherwise removes and recreates.

## Indicator language

**Named indicator sequence**:
A discoverable, semantically classified, ordered set of one-cell Unicode
frames whose declared order and endpoints are part of the visual contract.
_Avoid_: Inline frames, animation preset

**Indicator reference**:
A consumer's selection of one named indicator sequence with a compatible
semantic kind.
_Avoid_: Frame copy, sequence alias
