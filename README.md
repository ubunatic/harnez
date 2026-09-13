# harnez

Manage your [Claude Code](https://claude.ai/code) and Prime Agent harnesses declaratively from a
single `config.yaml`. One source of truth drives everything Claude Code and coding agents read:
permissions, effort levels, optional model overrides, hooks, AGENTS.md instructions, custom slash
commands, skills, and language doc copies. Apply is idempotent — run it as often as
you like; user-managed keys and unmanaged sections are never touched.

**Website:** <https://ubunatic.com/harnez> · **Repo:** <https://codeberg.org/ubunatic/harnez>

## What it manages

| Thing | Where |
|---|---|
| Effort level & optional model | `~/.claude/settings.json` |
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
make install          # builds and installs to ~/go/bin
```

System-wide:

```sh
make install-system   # installs to /usr/local/bin via sudo
```

After install the binary is self-contained: it embeds its own `config.yaml` and
command files, so `harnez apply` (no flags) applies the built-in config.

## Quick start

```sh
harnez apply           # apply embedded config to Claude, Gemini, Codex, and Prime Agent
harnez status          # show what is and isn't applied
harnez diff            # preview changes without writing
harnez usage           # show unified token, session, and quota status across agents
harnez usage --watch   # live TUI dashboard with real-time token velocity
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
target_dir: ~/.claude               # where Claude Code stores its config
skills_target: ~/.gemini/skills     # Gemini skills target
codex_skills_target: ~/.codex/skills # Codex skills target
claude_skills_target: ~/.claude/skills # Claude Code's own Agent Skills dir (auto-loaded)
prime_agent_target: ~/.prime/agent  # Prime rules, prompts, skills, and docs

# Docs to install globally on every apply
docs:
  - golang
  - bash
  - make
  - agentic-loop

# model: sonnet              # optional override; unmanaged by default so user/client chooses
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

# Custom slash commands written to ~/.claude/commands/ and ~/.prime/agent/prompts/
commands:
  - name: review
    description: "Review current branch against main"
    content: |
      Review the diff vs main. Check for: correctness, security, tests, docs.

  - name: domain-modeling
    description: "Actively build and sharpen the project's domain model, glossary, and ADRs"
    file: commands/domain-modeling.md   # body read from this file

# Skills written to ~/.gemini/skills/, ~/.codex/skills/, and ~/.prime/agent/skills/
skills:
  - name: sprint
    description: "Orchestrate a 5-phase agentic sprint loop"
    file: commands/sprint.md

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
      source: docs/lang/Go.md
      target: ~/.claude/docs/Go.md
      local: ./docs/Go.md
    bash:
      ref: "@docs/Bash.md"
      source: docs/lang/Bash.md
      target: ~/.claude/docs/Bash.md
      local: ./docs/Bash.md
```

## How each piece works

### `settings.json` — key merge, not replace

Claude Code writes `settings.json` itself (theme, plugins, auth).
`harnez` only touches the keys it owns (`effortLevel`, `permissions`, `hooks`,
`env`, `spinnerVerbs`, `mcpServers`, and `model` if explicitly set). If `model`
is omitted from `config.yaml`, user or client selection in `settings.json` is
left unmanaged. All other unmanaged keys are preserved on every apply.

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
Gemini, Codex, Claude Code, and Prime Agent skill target. The Claude Code target
(`claude_skills_target`, default `~/.claude/skills`) is a real, description-matched Agent Skill —
distinct from the always-manually-invoked `~/.claude/commands/<name>.md` slash command the same
entry also produces.

### Language & practice docs

Entries under `languages:` define source docs (e.g. `docs/lang/Go.md`, `docs/practices/IssueTracking.md`), global
installs for Claude and Prime Agent, and project copies
(`docs/Go.md`, `docs/IssueTracking.md`). Docs listed in the top-level `docs:` list (or passed via
`--docs`) are installed on `apply`.

## Issue Tracking & Priority Standards

`harnez` establishes a standardized, in-repository issue tracking convention across all managed projects (`issues/NNN-*.md`):

### Issue Priority Schema

Priorities define **scheduling urgency**:

| Priority | Level | Description | Target SLA / Lifecycle |
|---|---|---|---|
| **P0** | **Critical** | Blocker, data loss, security vulnerability, broken build, or invariant violation. Halts regular dev. | Immediate resolution ("stop the line"). |
| **P1** | **High** | Core functionality broken, major workflow impediment, high-urgency milestone deliverable. | Current sprint / next immediate release. |
| **P2** | **Medium** | Standard bug, normal feature, performance improvement, UX polish, non-blocking refactor. | Normal backlog prioritization. |
| **P3** | **Low** | Minor cosmetic polish, typo, nice-to-have suggestion, speculative idea, non-urgent doc fix. | Opportunistic. |

> **Priority vs. Severity**: *Severity* measures technical impact and damage (Critical, Major, Moderate, Minor). *Priority* measures scheduling urgency (P0, P1, P2, P3).

### Standard Ticket Metadata Header

Every ticket in `issues/NNN-kebab-case.md` begins with standard metadata headers:

```markdown
# NNN — Title of Issue

**Status**: Open | In Progress | Blocked — <reason> | Closed — resolved in <commit> | Draft
**Priority**: P0 (Critical) | P1 (High) | P2 (Medium) | P3 (Low)
**Severity**: Critical | Major | Moderate | Minor
**Category**: Bug | Feature | Architecture | Documentation | Performance | Refactor | Agentic Ergonomics
**Related**: [Doc / Ticket / Commit references]
```

- **Tracker Index**: `issues/README.md` indexes all active and archived tickets.
- **Tracker Linter**: `harnez status` validates table links, checks for status drift, and detects unindexed tickets.
- **Archiving**: Resolved tickets are moved to `issues/archive/` to keep active issue queues focused.

## Commands reference

| Command | Flags | What it does |
|---|---|---|
| `apply` | `-c` `-t` `-d` `--force-docs` | Sync global Claude, Gemini, Codex, and Prime Agent rules, prompts, skills, and docs |
| `init` | `-c` `-d` `-f` `--docs` `-m` `--summary` `--update` `--replace` `-y` | Set up a project: AGENTS.md, doc copies, Makefile targets |
| `diff` | `-c` `-t` `-e` `--capture-docs` `--out` | Preview changes without writing (`-e, --exit-code` exits with 1 on drift; `--capture-docs` writes report to inbox) |
| `scan-docs` | `-c <dir>` | Read-only scan of child projects for managed doc drift |
| `clean` | `-c` `-t` | Remove managed keys from `settings.json`; strip MD sections |
| `status` | `-c` `-t` | Print config summary and check which items are present on disk |
| `assess` | `[path]` `--json` | Fast code/doc metrics, token estimation, and repo feasibility report |
| `mode` | `[level]` `--status` `--clear` | Switch ConciseMode terseness level and sync AGENTS.local.md overlay |
| `distill` | `[hook|filter]` | Distill verbose command outputs for agent context conservation |
| `release` | `--bump` `--continue` `--dry-run` `-s` | Language-agnostic version bump, build, minisign signing, and forge publishing |
| `usage` | `--json` `--agent` `--offline` `-w` `-s` `--interval` | Show unified token, session, and quota status across AI coding agents (aliases: `quota`, `tokens`, `stats`) |
| `find <entity> <query…>` | `-d` | Fuzzy-text/filter query over repository-data entities (`issues` only in v1) |

All commands accept `-c <path>` (config file, default: embedded).

`apply` also accepts:

- `-t <dir>` — Claude config directory (default: `~/.claude`)
- `-d, --docs <name>[,<name>…]` — extra doc(s) to install globally (comma-separated or repeated)
- `--force-docs` — overwrite existing docs with bundled versions

`diff` also accepts:

- `-t <dir>` — Claude config directory (default: `~/.claude`)
- `-e, --exit-code` — exit with status 1 if drift or changes are detected (useful for CI/pre-commit checks)

`usage` also accepts:

- `--json` — output usage metrics in JSON format
- `--agent <name>` — filter to a specific agent (`claude`, `agy`, `codex`)
- `--offline` — disable live network queries and use local caches only
- `-w, --watch` — live-refresh the dashboard in place with a tokens/min velocity trend
- `-s, --summary` — print the compact dashboard once and exit
- `--interval <duration>` — refresh interval for `--watch` (default `15s`, minimum `10s`)

`init` also accepts:

- `-d <dir>` — project directory (default: `.`)
- `-f, --force` — allow initializing home directory, root, or non-coding directory
- `--docs <name>[,<name>…]` — doc(s) to set up in the project (comma-separated or repeated)
- `-m, --repo-mode <mode>` — repo git setup to note in AGENTS.md (`solo`, `fork`, `team`)
- `--summary` — run `claude -p` to generate an AI project summary in AGENTS.md
- `--update` — re-fetch and refresh the project summary (implies `--summary`)
- `--replace` — delete existing AGENTS.md and recreate from template before init
- `-y` — assume yes when reconciling Makefile targets (no prompt)

`find` also accepts:

- `-d <dir>` — repo root containing `issues/` (default: `.`)

`find issues` searches `issues/*.md` and `issues/archive/*.md` (excluding `issues/README.md`)
with a short query language: whitespace between terms is AND, `vram|gtt` within one term is
OR, and `status:<value>`/`is:<value>` (alias) filters on lifecycle — accepted values `open`,
`in-progress`, `blocked`, `closed`, `draft`. Filters and AND bind outside OR, so
`status:open vram|gtt` means `status:open AND (vram OR gtt)`. An unquoted `|` is a shell
pipeline operator, so quote OR queries:

```sh
harnez find issues status:open vram gtt
harnez find issues "status:open vram|gtt"
```

Matching is fuzzy (typo-tolerant via Damerau-Levenshtein, prefix-aware, deterministic) over
the title and body text, never a regex or full Boolean grammar; run `harnez find --help` for
the full normalization/ranking contract. Output is tab-separated
(`NUMBER\tRAW_STATUS\tPLAIN_TITLE\tPATH`), one match per line, with no header — zero matches
exits 0 silently, and an invalid entity/query exits non-zero with an actionable message. An
AND query is never silently relaxed to OR when it finds nothing.

### Project setup

`init` is the one command for setting up a project directory. It creates `AGENTS.md`
and the `CLAUDE.md` symlink, applies config-defined local sections (language conventions),
copies language docs locally, and injects standard Makefile targets — all in one step.

```sh
harnez init                              # AGENTS.md + CLAUDE.md symlink only
harnez init --docs golang,make          # + docs + Makefile targets
harnez init --docs golang --summary     # + AI-generated project summary
```

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
make install  # build then install to ~/go/bin
```

Smoke-test (build → apply → idempotency check → drift simulation → repair):

```sh
scripts/smoke-test.sh
```
