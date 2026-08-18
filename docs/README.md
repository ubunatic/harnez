# Docs — Evergreen Project Docs

In-depth references for decisions, architecture, and pitfalls specific to this codebase.
Not needed for routine coding; reach for these during investigations or design work.

| File | Topic |
|------|-------|
| [CLIDesign.md](CLIDesign.md) | apply vs init separation: scope, rationale, footgun avoided, design evolution |
| [CommandsPipeline.md](CommandsPipeline.md) | Claude commands and Prime prompts plus shared skills for Gemini, Codex, and Prime Agent |
| [LanguagePipeline.md](LanguagePipeline.md) | Language pipeline: docs install, template scaffolding, targets injection, Markers abstraction, lint |
| [Permissions.md](Permissions.md) | Claude Code permission model; Bash vs Read layers; grow-only caveat |


Copyable docs (installed to Claude and Prime Agent global dirs on `apply`, copied to projects via `--docs`) live in subdirs.
Generated copies land here (root) after `apply` / `init`.

**`docs/lang/`** — language/SDK/framework docs

| File | Topic |
|------|-------|
| [lang/Go.md](lang/Go.md) | Go conventions |
| [lang/Bash.md](lang/Bash.md) | Bash/Shell conventions |
| [lang/Make.md](lang/Make.md) | Makefile conventions |
| [lang/Git.md](lang/Git.md) | Git conventions |
| [lang/Rust.md](lang/Rust.md) | Rust conventions |
| [lang/Zig.md](lang/Zig.md) | Zig conventions |
| [lang/Cpp.md](lang/Cpp.md) | C/C++ conventions |
| [lang/Markdown.md](lang/Markdown.md) | Markdown conventions |
| [lang/GTK4.md](lang/GTK4.md) | GTK4/PyGObject conventions |

**`docs/other/`** — practice docs (no category yet)

| File | Topic |
|------|-------|
| [other/Canary.md](other/Canary.md) | Canary-first development: probe external mechanisms before building |
| [other/Spec.md](other/Spec.md) | Spec-driven architecture: YAML spec files as single source of truth |

**`docs/studies/`** — case studies & background reports (reference material for future generic docs)

| File | Topic |
|------|-------|
| [studies/GoRelease.md](studies/GoRelease.md) | Release pipeline case study (goreleaser consolidation) |
| [studies/Worktrees.md](studies/Worktrees.md) | Parallel worktrees case study (go.mod races, dependencies) |
| [studies/2026-08-16-harnez-migration-and-workspace-unification.md](studies/2026-08-16-harnez-migration-and-workspace-unification.md) | Harnez migration, Spec generalization & workspace diagnostics case study |
| [studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md](studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md) | Multi-agent token, session & quota monitoring case study (Claude Code, AGY, Codex) |

**`docs/feedback/`** — agentic retrospectives & harness feedback reports

| File | Topic |
|------|-------|
| [feedback/2026-08-18-agentic-extraction-blindspots-and-harness-gaps.md](feedback/2026-08-18-agentic-extraction-blindspots-and-harness-gaps.md) | Subagent domain extraction blindspots, wrapper traps, and proposed harnez features |

**`docs/proposed/`** — staging area for docs that may become copyable (no install mechanics yet).
