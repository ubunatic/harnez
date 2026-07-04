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

Copyable docs (installed to `~/.claude/docs/` on `apply`, copied to projects via `--doc`) live in subdirs:
- `docs/lang/` — language/SDK/framework docs (Go, Bash, Make, Git, Rust, Cpp, Markdown, GTK4)
- `docs/other/` — practice docs not yet forming a category (Canary)
- `docs/proposed/` — staging area for candidate copyable docs

Generated copies of those docs (`Go.md`, `Bash.md`, etc.) land here after `apply`.
