# harnez

Manage your [Claude Code](https://claude.ai/code) and Prime Agent harnesses declaratively from a
single `config.yaml`. One source of truth drives everything Claude Code and coding agents read:
permissions, model settings, hooks, AGENTS.md instructions, custom slash
commands, skills, and language doc copies. Apply is idempotent — run it as often as
you like; user-managed keys and unmanaged sections are never touched.

**Website:** <https://ubunatic.com/harnez> · **Repo:** <https://codeberg.org/ubunatic/harnez>

## What it manages

| Thing | Where |
|---|---|
| Model & effort level | `~/.claude/settings.json` |
| Permission allow/deny lists | `~/.claude/settings.json` |
| Lifecycle hooks (Stop, etc.) | `~/.claude/settings.json` |
| Environment variables | `~/.claude/settings.json` |
| Spinner verbs | `~/.claude/settings.json` |
| MCP server definitions | `~/.claude/settings.json` |
| Global agent instructions | `~/.claude/CLAUDE.md`, `~/.prime/agent/AGENTS.md` (managed sections) |
| Per-project instructions | `./AGENTS.md` (managed sections, `CLAUDE.md` symlink) |
| Custom slash commands/prompts | `~/.claude/commands/<name>.md`, `~/.prime/agent/prompts/<name>.md` |
| Agent skills | `~/.gemini/skills/`, `~/.codex/skills/`, `~/.prime/agent/skills/` |
| Language coding-style docs | `~/.claude/docs/`, `~/.prime/agent/docs/` + project copies |

## Installation

```sh
go install ubunatic.com/harnez@latest
```

Or from source:

```sh
git clone https://codeberg.org/ubunatic/harnez
cd harnez
make install          # builds and installs to /usr/local/bin
```

After install the binary is self-contained: it embeds its own `config.yaml` and
command files, so `harnez apply` (no flags) applies the built-in config.

## Quick start

```sh
harnez apply           # apply embedded config to Claude, Gemini, Codex, and Prime Agent
harnez status          # show what is and isn't applied
harnez diff            # preview changes without writing
harnez clean           # remove all managed blocks/keys
```

With a custom config file:

```sh
harnez apply -c myconfig.yaml -t ~/.claude
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
prime_agent_target: ~/.prime/agent  # Prime rules, prompts, skills, and docs

# Docs to install on every apply
docs:
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
`harnez` only touches the keys it owns (`model`, `effortLevel`,
`permissions`, `hooks`, `env`, `spinnerVerbs`, `mcpServers`).  All other keys
are preserved on every apply.

### CLAUDE.md / AGENTS.md — managed sections

Instructions are written inside HTML-comment markers:

```
<!-- harnez:begin Section Name -->
...content managed by harnez...
<!-- harnez:end Section Name -->
```

Sections outside these markers are never modified. Multiple sections can
coexist in the same file, each independently updated or removed.

**Symlink convention** — `AGENTS.md` is the canonical file (readable by any
agent); `CLAUDE.md` is a symlink so Claude Code finds it too. Globally,
`~/.claude/CLAUDE.md` is real and `~/AGENTS.md` links to it; the same managed
sections are also written to Prime Agent's canonical `~/.prime/agent/AGENTS.md`.

### Slash commands and Skills

Each entry under `commands:` produces both a Claude command and a Prime Agent prompt template.
Each entry under `skills:` produces Agent Skills-compatible `SKILL.md` files for every configured
Gemini, Codex, and Prime Agent skill target.

### Language docs

Entries under `languages:` define a source doc (e.g. `docs/lang/Go.md`), global
installs for Claude and Prime Agent, and a project copy
(`docs/Go.md`). Docs listed in the top-level `docs:` list (or passed via
`--docs`) are installed on `apply`.

## Commands reference

| Command | Flags | What it does |
|---|---|---|
| `apply` | `-c` `-t` `-d` `--force-docs` | Sync global Claude and Prime Agent rules, prompts, skills, and docs |
| `init` | `-c` `-d` `--docs` `--summary` | Set up a project: AGENTS.md, doc copies, Makefile targets |
| `diff` | `-c` `-t` | Preview changes without writing (uses `diff -u`) |
| `clean` | `-c` `-t` | Remove managed keys from `settings.json`; strip MD sections |
| `status` | `-c` `-t` | Print config summary and check which items are present on disk |
| `tools` | `status`, `install voice-input` | Read-only capability status or explicit, planned OS-tool setup |

All commands accept `-c <path>` (config file, default: embedded).

`apply` also accepts:

- `-t <dir>` — Claude config directory (default: `~/.claude`)
- `-d, --docs <name>[,<name>…]` — extra doc(s) to install globally (comma-separated or repeated)
- `--force-docs` — overwrite existing docs with bundled versions

`init` also accepts:

- `-d <dir>` — project directory (default: `.`)
- `--docs <name>[,<name>…]` — doc(s) to set up in the project (comma-separated or repeated)
- `-m, --repo-mode <mode>` — repo git setup to note in AGENTS.md (`solo`, `fork`, `team`)
- `--summary` — run `claude -p` to generate an AI project summary in AGENTS.md
- `--update` — re-fetch and refresh the project summary (implies `--summary`)
- `--replace` — delete existing AGENTS.md and recreate from template before init
- `-y` — assume yes when reconciling Makefile targets (no prompt)

### Project setup

`init` is the one command for setting up a project directory. It creates `AGENTS.md`
and the `CLAUDE.md` symlink, applies config-defined local sections (language conventions),
copies language docs locally, and injects standard Makefile targets — all in one step.

```sh
harnez init                              # AGENTS.md + CLAUDE.md symlink only
harnez init --docs golang,make          # + docs + Makefile targets
harnez init --docs golang --summary     # + AI-generated project summary
```

## Optional workstation tools

OS-level capabilities are deliberately separate from `apply` and `init`:

```sh
harnez tools                              # read-only catalog and status
harnez tools status voice-input           # read-only readiness probes
harnez tools install voice-input --dry-run # exact plan, no network or changes
```

Voice input defaults to user-local, offline, no-sudo setup. The Fedora 44 GNOME Wayland
hardware canary has fully passed (mic capture, transcription, toggle shortcut, and direct
text injection all verified); the `install` command itself is still a plan-and-safety-gate
pending a converging installer implementation of the verified manual setup; see
[`docs/VoiceInput.md`](docs/VoiceInput.md).

## Drift repair

If permissions or settings drift,
re-running `make apply` restores the managed keys while leaving everything else
alone. Use `make diff` first to see exactly what would change.

## Development

```sh
make build    # compile ./harnez
make test     # go vet + go test
make apply    # build then apply repo config.yaml to ~/.claude
make diff     # build then preview changes
make install  # build then install to /usr/local/bin
```

Smoke-test (build → apply → idempotency check → drift simulation → repair):

```sh
scripts/smoke-test.sh
```
