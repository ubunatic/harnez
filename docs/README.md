# Docs — Evergreen Project Docs

In-depth references for decisions, architecture, and pitfalls.
Not needed for routine coding; reach for these during investigations or design work.

| File | Topic |
|------|-------|
| [CommandsPipeline.md](CommandsPipeline.md) | Claude commands (config.yaml → ~/.claude/commands/) and Antigravity skills (→ ~/.gemini/skills/) |
| [LanguagePipeline.md](LanguagePipeline.md) | Language pipeline: docs install, template scaffolding, targets injection, Markers abstraction, lint |
| [Permissions.md](Permissions.md) | Claude Code permission model; Bash vs Read layers; grow-only caveat |
| [Worktrees.md](Worktrees.md) | Parallel worktree agents: what worked, go.mod races, Haiku API staleness |
| [Go.md](Go.md) | Go conventions (generated from docs/src/Go.md) |
| [Bash.md](Bash.md) | Bash conventions (generated from docs/src/Bash.md) |
| [Make.md](Make.md) | Makefile conventions (generated from docs/src/Make.md) |

Language docs (`Go.md`, `Bash.md`, `Make.md`) are installed to `~/.claude/docs/` on apply
and also copied into project repos via `-l` flags. Edit their sources in `docs/src/`.
