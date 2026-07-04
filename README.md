# claudeconfig

Manage your [Claude Code](https://claude.ai/code) setup declaratively from a
single `config.yaml`.  One source of truth drives everything Claude Code reads:
permissions, model settings, hooks, CLAUDE.md instructions, custom slash
commands, and language doc symlinks.  Apply is idempotent — run it as often as
you like; user-managed keys and unmanaged sections are never touched.

## What it manages

| Thing | Where |
|---|---|
| Model & effort level | `~/.claude/settings.json` |
| Permission allow/deny lists | `~/.claude/settings.json` |
| Lifecycle hooks (Stop, etc.) | `~/.claude/settings.json` |
| Environment variables | `~/.claude/settings.json` |
| Spinner verbs | `~/.claude/settings.json` |
| MCP server definitions | `~/.claude/settings.json` |
| Global agent instructions | `~/.claude/CLAUDE.md` (managed sections) |
| Per-project instructions | `./AGENTS.md` (managed sections, `CLAUDE.md` symlink) |
| Custom slash commands | `~/.claude/commands/<name>.md` |
| Language coding-style docs | `~/.claude/docs/<lang>.md` + project symlinks |

## Installation

```sh
go install ubunatic.com/claudeconfig@latest
```

Or from source:

```sh
git clone https://codeberg.org/ubunatic/claudeconfig
cd claudeconfig
make install          # builds and installs to /usr/local/bin
```

After install the binary is self-contained: it embeds its own `config.yaml` and
command files, so `claudeconfig apply` (no flags) applies the built-in config.

## Quick start

```sh
claudeconfig apply           # apply embedded config to ~/.claude
claudeconfig status          # show what is and isn't applied
claudeconfig diff            # preview changes without writing
claudeconfig clean           # remove all managed blocks/keys
```

With a custom config file:

```sh
claudeconfig apply -c myconfig.yaml -t ~/.claude -p /path/to/project
```

Or via Make (builds first, then runs):

```sh
make apply
make diff
make clean
make status
```

## config.yaml overview

```yaml
target_dir: ~/.claude        # where Claude Code stores its config

# Language docs to install on every apply
langs:
  - golang
  - bash
  - make

model: sonnet                # model alias (sonnet / opus / haiku)
effort: medium               # effortLevel written to settings.json

# Merged into settings.json permissions (user keys preserved)
permissions:
  allow:
    - "Bash(git *)"
    - "Bash(make *)"
    - "Read(~/projects/**)"
  deny: []

# Lifecycle hooks
hooks:
  - event: Stop
    command: "ffplay -nodisp -autoexit /usr/share/sounds/freedesktop/stereo/window-attention.oga 2>/dev/null || true"

# Spinner verbs shown while Claude thinks
verbs:
  - "🤖"

# Environment variables injected into Claude Code sessions
env:
  DEBUG: "false"

# MCP server definitions (empty = none)
mcp_servers: []

# Custom slash commands written to ~/.claude/commands/
commands:
  - name: review
    description: "Review current branch against main"
    content: |
      Review the diff vs main. Check for: correctness, security, tests, docs.

  - name: domain-modeling
    description: "Actively build and sharpen the project's domain model, glossary, and ADRs"
    file: commands/domain-modeling.md   # body read from this file

# CLAUDE.md / AGENTS.md sections
agents_md:
  global:
    target: ~/.claude/CLAUDE.md
    symlink: ~/AGENTS.md
    sections:
      - name: My Rules
        content: |
          - Always use absolute paths in global instructions.

  local:
    target: AGENTS.md
    symlink: CLAUDE.md
    sections:
      - name: Language Conventions
        content: |
          - Go/Golang @docs/Go.md
          - Bash/Shell @docs/Bash.md

  # Language doc registry
  languages:
    golang:
      ref: "@docs/Go.md"
      source: docs/Go.md
      target: ~/.claude/docs/Go.md
      symlink: ./docs/Go.md
    bash:
      ref: "@docs/Bash.md"
      source: docs/Bash.md
      target: ~/.claude/docs/Bash.md
      symlink: ./docs/Bash.md
```

## How each piece works

### `settings.json` — key merge, not replace

Claude Code writes `settings.json` itself (theme, plugins, auth).
`claudeconfig` only touches the keys it owns (`model`, `effortLevel`,
`permissions`, `hooks`, `env`, `spinnerVerbs`, `mcpServers`).  All other keys
are preserved on every apply.

### CLAUDE.md / AGENTS.md — managed sections

Instructions are written inside HTML-comment markers:

```
<!-- claudeconfig:begin Section Name -->
...content managed by claudeconfig...
<!-- claudeconfig:end Section Name -->
```

Sections outside these markers are never modified.  Multiple sections can
coexist in the same file, each independently updated or removed.

**Symlink convention** — `AGENTS.md` is the canonical file (readable by any
agent); `CLAUDE.md` is a symlink so Claude Code finds it too.  Same pattern
globally: `~/.claude/CLAUDE.md` is real; `~/AGENTS.md` links to it.

### Slash commands

Each entry under `commands:` produces `~/.claude/commands/<name>.md` with a
YAML frontmatter `description:` line and the prompt body.  Body can be inline
(`content:`) or read from a file (`file:`).

### Language docs

Entries under `languages:` define a source doc (e.g. `docs/Go.md`), a global
install path (`~/.claude/docs/Go.md`), and an optional project symlink
(`./docs/Go.md`).  Languages listed in `langs:` (or passed via `--lang`) are
installed on `apply`.  The project symlink lets `@docs/Go.md` resolve locally
without duplicating the file.

## Commands reference

| Command | Flags | What it does |
|---|---|---|
| `apply` | `-c` `-t` `-l` `--force-docs` | Sync global `~/.claude` config: settings, hooks, commands, lang docs |
| `init` | `-c` `-d` `-l` `--summary` | Set up a project: AGENTS.md, lang doc copies, Makefile targets |
| `diff` | `-c` `-t` | Preview changes without writing (uses `diff -u`) |
| `clean` | `-c` `-t` | Remove managed keys from `settings.json`; strip MD sections |
| `status` | `-c` `-t` | Print config summary and check which items are present on disk |

All commands accept `-c <path>` (config file, default: embedded).

`apply` also accepts:

- `-t <dir>` — Claude config directory (default: `~/.claude`)
- `-l <lang>` — extra language doc to install globally (repeatable)
- `--force-docs` — overwrite existing language docs with bundled versions

`init` also accepts:

- `-d <dir>` — project directory (default: `.`)
- `-l <lang>` — language(s) to set up in the project (repeatable)
- `--summary` — run `claude -p` to generate an AI project summary in AGENTS.md
- `--replace` — delete existing AGENTS.md and recreate from template before init

### Project setup

`init` is the one command for setting up a project directory.  It creates `AGENTS.md`
and the `CLAUDE.md` symlink, applies config-defined local sections (language conventions),
copies language docs locally, and injects standard Makefile targets — all in one step.

```sh
claudeconfig init                        # AGENTS.md + CLAUDE.md symlink only
claudeconfig init -l golang -l make     # + lang docs + Makefile targets
claudeconfig init -l golang --summary   # + AI-generated project summary
```

## Drift repair

If permissions or settings drift (Claude Code adds/removes something),
re-running `make apply` restores the managed keys while leaving everything else
alone.  Use `make diff` first to see exactly what would change.

## Development

```sh
make build    # compile ./claudeconfig
make test     # go vet + go test
make apply    # build then apply repo config.yaml to ~/.claude
make diff     # build then preview changes
make install  # build then install to /usr/local/bin
```

Smoke-test (build → apply → idempotency check → drift simulation → repair):

```sh
scripts/smoke-test.sh
```
