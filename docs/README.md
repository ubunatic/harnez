# Docs — Evergreen Project Docs

In-depth references for decisions, architecture, and pitfalls specific to this codebase.
Not needed for routine coding; reach for these during investigations or design work.

| File | Topic |
|------|-------|
| [CLIDesign.md](CLIDesign.md) | apply vs init separation: scope, rationale, footgun avoided, design evolution |
| [CommandsPipeline.md](CommandsPipeline.md) | Claude commands (config.yaml → ~/.claude/commands/) and shared skills (→ ~/.gemini/skills/, ~/.codex/skills/) |
| [LanguagePipeline.md](LanguagePipeline.md) | Language pipeline: docs install, template scaffolding, targets injection, Markers abstraction, lint |
| [Permissions.md](Permissions.md) | Claude Code permission model; Bash vs Read layers; grow-only caveat |
| [Worktrees.md](Worktrees.md) | Parallel worktree agents: what worked, go.mod races, Haiku API staleness |

Copyable docs (installed to `~/.claude/docs/` on `apply`, copied to projects via `--docs`) live in subdirs.
Generated copies land here (root) after `apply`.

**`docs/lang/`** — language/SDK/framework docs

| File | Topic |
|------|-------|
| [lang/Go.md](lang/Go.md) | Go conventions |
| [lang/Bash.md](lang/Bash.md) | Bash/Shell conventions |
| [lang/Make.md](lang/Make.md) | Makefile conventions |
| [lang/Git.md](lang/Git.md) | Git conventions |
| [lang/Rust.md](lang/Rust.md) | Rust conventions |
| [lang/Cpp.md](lang/Cpp.md) | C/C++ conventions |
| [lang/Markdown.md](lang/Markdown.md) | Markdown conventions |
| [lang/GTK4.md](lang/GTK4.md) | GTK4/PyGObject conventions |

**`docs/other/`** — practice docs (no category yet)

| File | Topic |
|------|-------|
| [other/Canary.md](other/Canary.md) | Canary-first development: probe external mechanisms before building |
| [other/Spec.md](other/Spec.md) | Spec-driven architecture: YAML spec files as single source of truth |

**`docs/proposed/`** — staging area for docs that may become copyable (no install mechanics yet).
