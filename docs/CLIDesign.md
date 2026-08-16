---
title: CLI Design: apply vs init
weight: 30
---

# CLI Design — apply vs init separation

Documents the command structure, the design decision behind it, and the pitfalls it avoids.

## Command responsibilities

| Command | Scope | What it touches |
|---------|-------|-----------------|
| `apply` | Global (`~/.claude`) | `settings.json`, `CLAUDE.md`, `commands/`, `docs/<name>.md` |
| `init`  | Project (cwd / `-d`) | `AGENTS.md`, `CLAUDE.md` symlink, `docs/<name>.md` copy, `Makefile` |
| `diff`  | Global | Preview of what `apply` would change |
| `clean` | Global | Remove managed keys / strip MD sections |
| `status`| Global | Config summary + applied-state checks |

`apply` and `init` operate on disjoint flag surfaces by design. `apply` takes `-t`
(Claude config dir); `init` takes `-d` (project dir). They cannot be confused.

## Why the separation matters

Before the split, `apply` accepted both `-t <claude-dir>` and `-p <project-dir>`.
The flags look symmetric but have opposite blast radii:

- `-t .` would write `settings.json`, `CLAUDE.md`, and `commands/` into the **cwd**,
  silently corrupting a project directory.
- `-p /wrong/path` would scaffold Makefile targets in the wrong repo.

A user asking "do I need `-p .`?" revealed this footgun. The fix was structural:
project-local work lives in `init`, which defaults to cwd and has no `-t` flag at all.

## Design evolution

1. **Phase 1** — `apply -p <dir>` did everything: global sync + project-local wiring.
2. **Phase 2** — Added `--setup` flag to gate Makefile injection (opt-in).
3. **Phase 3** — Recognised the deeper issue: `apply` mixes two orthogonal concerns.
   Evaluated three options:
   - A: Validate-only (error if `--setup` without `-p`)
   - B: Merge project work into `init` ← **chosen**
   - C: New `project` subcommand
4. **Current** — `apply` is global-only; `init -l <lang>` owns all project setup.

Option B was chosen because `init` already existed as a "bootstrap this project" command
and extending it was natural, while `apply` became a clean "maintain my Claude install"
command. Issue #001 (diff/clean wrong path when `--project` set) was resolved as a
by-product: there is no `--project` flag anymore.

## init flow

```
harnez init [-d <dir>] [--docs <name>...]
        │
        ├── create AGENTS.md (template) if absent
        ├── create CLAUDE.md symlink → AGENTS.md
        ├── auto-detect docs (default:auto/true entries in config.yaml)
        ├── apply cfg.AgentsMD.Local sections (Language Conventions, etc.)
        └── for each --docs <name> (explicit + auto-detected):
                ├── copy docs/<name>.md locally  (for @docs/ refs)
                ├── scaffold Makefile from template  (if no Makefile)
                └── inject targets block into existing Makefile
```

All steps are idempotent. Running `init --docs golang` twice is safe.

## apply flow

```
harnez apply [-t <dir>] [-d <name>...] [--force-docs]
        │
        ├── merge managed keys into settings.json
        ├── write ~/.claude/CLAUDE.md managed sections
        ├── create ~/AGENTS.md symlink → ~/.claude/CLAUDE.md
        ├── write ~/.claude/commands/<name>.md for each command
        ├── write ~/.gemini/skills/<name>/SKILL.md for each skill
        ├── write ~/.codex/skills/<name>/SKILL.md for each skill when configured
        └── install ~/.claude/docs/<name>.md for each doc
```

## Flag shorthands

`apply --docs` has shorthand `-d`. `init --docs` does **not** — `-d` is already taken by
`--dir`. When adding new flags to either command, check for shorthand conflicts before
committing to a letter.

## Known gaps

- `diff` and `clean` are global-only and have no awareness of project Makefiles — see issue #009.
- `status` does not check whether the project Makefile targets block is present.
