# claudeconfig — Claude Code Config Manager

A Go CLI tool that manages Claude Code configuration files declaratively, in the same spirit as [vimconfig](../vimconfig): a single `config.yaml` drives all outputs, a managed-block strategy lets user edits coexist with generated content, and an `apply` command makes everything idempotent.

---

## What Claude Code has that needs managing

| File | Scope | Format |
|------|-------|--------|
| `~/.claude/settings.json` | Global | JSONC |
| `~/.claude/keybindings.json` | Global | JSONC |
| `~/.claude/CLAUDE.md` | Global instructions | Markdown |
| `~/.claude/docs/*.md` | Shared language/rule docs | Markdown |
| `~/.claude/commands/*.md` | Custom slash commands | Markdown |
| `AGENTS.md` + `CLAUDE.md` symlink | Project-local instructions | Markdown |

The tool manages global files (`~/.claude/`) by default; `--target` or `target_dir` in config redirects output for project-scoped configs.

---

## Architecture (mirrors vimconfig)

```
config.yaml           ← single source of truth
    │
    ▼
claudeconfig (Go CLI)
    ├── config.go     ← Config structs + YAML loader
    ├── apply.go      ← generators: settings, keybindings, agents_md, commands
    ├── deps.go       ← dependency checker for MCP servers / required tools
    └── main.go       ← Cobra CLI entry point
```

### Managed blocks

JSONC files use `//` comment markers:

```jsonc
// claudeconfig:begin settings
"permissions": { ... },
"hooks": { ... },
"spinnerVerbs": { ... }
// claudeconfig:end settings
```

Markdown files (CLAUDE.md, AGENTS.md, commands) use HTML comment markers:

```markdown
<!-- claudeconfig:begin instructions -->
...generated content...
<!-- claudeconfig:end instructions -->
```

`applySection()` (same logic as vimconfig):
1. Read the target file.
2. If managed markers exist → replace only that block.
3. Otherwise → append the block.
4. Write back. Idempotent, preserves surrounding user content.

---

## Config schema (`config.yaml`)

```yaml
target_dir: ~/.claude        # default output dir

model: sonnet                # short alias, resolved to full model ID
effort: medium               # effortLevel in settings.json

permissions:
  allow:
    - "Bash(git:*)"
    - "Read"
  deny: []

hooks:
  - event: Stop              # PostToolUse | Stop | PreToolUse | Notification
    command: "notify-send 'Claude stopped'"
  - event: PostToolUse
    matcher: "Bash"
    command: "echo 'ran bash' >> ~/.claude/audit.log"

verbs:                       # spinnerVerbs in settings.json (mode: replace)
  - "🤖"

env:
  DEBUG: "false"

keybindings:
  - key: "ctrl+y"
    action: acceptSuggestion

mcp_servers:
  - name: filesystem
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/uwe"]
    env: {}

commands:
  - name: review
    description: "Review current branch against main"
    content: |
      Review the diff vs main. Check for: correctness, security, tests, docs.

agents_md:
  global:
    target: ~/.claude/CLAUDE.md   # file to write
    symlink: ~/AGENTS.md          # symlink pointing at target
    sections:
      - name: Instructions Hierarchy
        content: |
          ...

  local:
    target: AGENTS.md             # canonical project file
    symlink: CLAUDE.md            # symlink → AGENTS.md in project root
    sections:
      - name: Language Conventions
        content: |
          ...

  languages:                      # use --lang <name> when applying
    golang:
      ref: "@docs/Go.md"          # token used in section content
      source: docs/Go.md          # bundled doc in this repo
      target: ~/.claude/docs/Go.md  # install destination
      symlink: ./docs/Go.md       # project-local symlink → target
    bash:
      ref: "@docs/Bash.md"
      source: docs/Bash.md
      target: ~/.claude/docs/Bash.md
      symlink: ./docs/Bash.md
```

---

## Commands

### `claudeconfig apply [-c config.yaml] [-t ~/.claude]`

Generates and writes:
1. `settings.json` — permissions, hooks, env, spinnerVerbs, MCP servers, model, effortLevel
2. `keybindings.json` — key bindings
3. `~/.claude/CLAUDE.md` — global managed sections (`agents_md.global`)
4. `AGENTS.md` + `CLAUDE.md` symlink — local managed sections (`agents_md.local`)
5. `~/.claude/docs/<lang>.md` — language docs copied from `source`, symlinked into project
6. `commands/<name>.md` — one file per custom command

### `claudeconfig diff [-c config.yaml] [-t ~/.claude]`

Shows a unified diff of what `apply` would change in each managed block, without writing.

### `claudeconfig status [-c config.yaml] [-t ~/.claude]`

Shows config summary and which managed blocks are present on disk.

### `claudeconfig clean [-c config.yaml] [-t ~/.claude]`

Removes all managed blocks written by `apply`, leaving surrounding user content intact.

### `claudeconfig deps [--dry-run]`

Checks MCP server dependencies (node/npx, python/uv, go), reports missing packages, optionally installs via apt.

---

## Implementation phases

### Phase 1 — Scaffold + apply settings.json ✓

- [x] `go mod init`
- [x] `go get github.com/spf13/cobra gopkg.in/yaml.v3`
- [x] Config structs + YAML loader (`config.go`)
- [x] `applySection()` / `diffSection()` / `cleanSection()` for JSONC (`apply.go`)
- [x] Generate `settings.json` (permissions, hooks, env, spinnerVerbs)
- [x] `apply`, `diff`, `clean` commands in `main.go`
- [x] `Makefile` with `help`, `build`, `install`, `apply`, `diff`, `clean`, `status`, `test`

### Phase 2 — Keybindings + agents_md ✓

- [x] Generate `keybindings.json` from `keybindings` list
- [x] `applySectionMD()` variant with HTML comment markers for Markdown files
- [x] Generate `~/.claude/CLAUDE.md` from `agents_md.global.sections`
- [x] Generate `AGENTS.md` + `CLAUDE.md` symlink from `agents_md.local`
- [x] `status` command (`status.go`) — port from vimconfig

### Phase 3 — Custom commands + MCP servers + language docs ✓

- [x] Generate `commands/<name>.md` files
- [x] Add MCP server block to `settings.json` generator
- [x] Copy language docs (`source` → `target`) and create project symlinks
- [x] `--lang <name>` flag to apply only specific language docs
- [x] Env var interpolation (`$VAR` passed through as-is)

### Phase 4 — Model/effort resolution ✓

- [x] Short alias map: `sonnet` → `claude-sonnet-4-6`, `opus` → `claude-opus-4-7`, etc.
- [x] Effort alias map: `low` / `medium` / `high` → `effortLevel` values
- [x] Emit `model` and `effortLevel` keys in `settings.json`

### Phase 5 — deps command

- [ ] Runtime → apt-package mapping table (node/npm, python3, go, etc.)
- [ ] `deps check` logic: scan MCP server commands, map to packages, `dpkg -l` check
- [ ] `deps install` with `--dry-run` flag

### Phase 6 — Schema validation (optional)

- [ ] Fetch official schema: `curl -o schema/claude-code-settings.schema.json https://json.schemastore.org/claude-code-settings.json`
- [ ] Validate permission rules and hook event names before writing
- [ ] `make update-schema` target to refresh pinned copy

---

## Key design decisions

**Single managed block for settings.json** — The entire claudeconfig-owned content is one `// claudeconfig:begin settings` / `// claudeconfig:end settings` block. User keys outside the block are untouched. Simpler than per-key blocks; the generator owns all its keys as a unit.

**AGENTS.md as canonical, CLAUDE.md as symlink** — Project instructions live in `AGENTS.md` (works with any agent tool); `CLAUDE.md` is just a symlink so Claude Code finds it automatically. Same pattern applied globally: `~/.claude/CLAUDE.md` is the real file, `~/AGENTS.md` symlinks to it.

**Language docs install globally, symlink locally** — Docs live in `~/.claude/docs/` (shared across projects). Each project gets a `./docs/<lang>.md` symlink so `@docs/Go.md` refs resolve locally first, then fall back to the global copy.

**Short model/effort aliases** — `model: sonnet` and `effort: medium` are human-friendly; the generator resolves them to the full Claude Code setting values before writing.

**spinnerVerbs mode: replace** — `verbs` list fully replaces the default verb set. Use `mode: append` if mixing with defaults is ever needed.

**Env var interpolation** — MCP server env values starting with `$` are emitted literally in JSON; the shell resolves them at MCP-server-launch time, keeping secrets out of config.yaml.

**Mirrors vimconfig closely** — Same file layout, same `applySection`/`diffSection`/`cleanSection` pattern, same Makefile structure, same Cobra CLI shape.
