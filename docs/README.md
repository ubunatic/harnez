# Docs — Evergreen Project Docs

In-depth references for decisions, architecture, and pitfalls specific to this codebase.
Not needed for routine coding; reach for these during investigations or design work.

> [!TIP]
> **Context Discipline**: see `docs/practices/AgenticLoop.md` Invariant 6 (Context Discipline & Range-Bounded Ingestion) for the canonical rule.

| File | Topic & Consultation Trigger |
|------|------------------------------|
| [CLIDesign.md](CLIDesign.md) | apply vs init separation: scope, rationale, footgun avoided, design evolution (consult before modifying CLI command flags) |
| [CommandsPipeline.md](CommandsPipeline.md) | Claude commands and Prime prompts plus shared skills for Gemini, Codex, and Prime Agent (consult when changing command pipelines) |
| [HookRewritePattern.md](HookRewritePattern.md) | Two-stage PreToolUse hook pattern (rewrite now, capture later): `<feature> hook` vs. `<feature>` wrapper, `harnez distill hook` as reference implementation, plus two hard-won constraints — hooks on the same matcher don't compose (last-to-finish wins) and a rewritten command must stay one shell token (consult before adding any new agent-hook-driven feature) |
| [LanguagePipeline.md](LanguagePipeline.md) | Language pipeline: docs install, template scaffolding, targets injection, Markers abstraction, lint |
| [Permissions.md](Permissions.md) | Claude Code permission model; Bash vs Read layers; grow-only caveat (consult when updating permission schemas) |


Copyable docs (installed to Claude and Prime Agent global dirs on `apply`, copied to projects via `--docs`) live in subdirs.
Generated copies land here (root) after `apply` / `init`.

**`docs/lang/`** — language/SDK/framework docs *(High-level rule summaries are already bundled in AGENTS.md; read files only for detailed templates or syntax nuances)*

| File | Topic | Read Trigger / Scope |
|------|-------|----------------------|
| [lang/Go.md](lang/Go.md) | Go conventions | Bounded read only for Cobra flag idioms, error wrapping conventions, or test templates |
| [lang/Bash.md](lang/Bash.md) | Bash/Shell conventions | Read for exact `if test` construct syntax or formatting rules |
| [lang/Make.md](lang/Make.md) | Makefile conventions | Read for ⚙️ sentinel phony mechanics and help-target recipes |
| [lang/Git.md](lang/Git.md) | Git conventions | Read for commit message format and worktree workflows |
| [lang/Rust.md](lang/Rust.md) | Rust conventions | Read for safe Rust conventions and clippy guidelines |
| [lang/Zig.md](lang/Zig.md) | Zig conventions | Read for Zig 0.16.0 allocator idioms and build.zig patterns |
| [lang/Cpp.md](lang/Cpp.md) | C/C++ conventions | Read for ccache setup and modern C++17 conventions |
| [lang/Markdown.md](lang/Markdown.md) | Markdown conventions | Read for naming guidelines (PascalCase evergreen vs kebab-case ephemeral) |
| [lang/GTK4.md](lang/GTK4.md) | GTK4/PyGObject conventions | Read for PyGObject signal handling and UI patterns |

**`docs/practices/`** — copyable practice and engineering workflow docs

| File | Topic | Read Trigger / Scope |
|------|-------|----------------------|
| [practices/AgenticLoop.md](practices/AgenticLoop.md) | Agentic Loop Practices: 5-phase sprint workflow (Advisory -> Dev -> Review -> Hygiene -> Retro), zero zombie guarantee | Summary active in AGENTS.md. Read for phase invariants, review checklists, and anti-patterns |
| [practices/IssueTracking.md](practices/IssueTracking.md) | Issue Tracking Practices: Standardized issue priority schema (P0–P3), metadata headers, and tracker lifecycle | Summary active in AGENTS.md. Read for priority definitions, severity vs priority distinctions, and ticket schemas |
| [practices/ConciseMode.md](practices/ConciseMode.md) | ConciseMode: 3 graded output-terseness tiers (Lite, Standard, Ultra) for slow/local-inference pairing | Opt-in (`init --docs concise-mode`). Read when tuning agent verbosity for low-TPS hardware |
| [practices/GoRelease.md](practices/GoRelease.md) | Release Pipeline (language-agnostic): `harnez release`, `version.yaml`, GoReleaser v2, non-interactive minisign (-W), Forgejo `has_releases` | Copyable doc (`init --docs gorelease`). Read when setting up or troubleshooting release automation, Go or not |
| [practices/DeploymentTransparency.md](practices/DeploymentTransparency.md) | Deployment Transparency: 3-state grounding rule (Local / Deployed Artifact / Active Daemon state) for remote deployment pairing | Copyable doc. Read before declaring remote deployment/scheduling status, or when pairing on live infra provisioning |

**`docs/other/`** — practice docs (no category yet)

| File | Topic | Read Trigger / Scope |
|------|-------|----------------------|
| [other/Canary.md](other/Canary.md) | Canary-first development: probe external mechanisms before building | Read when designing external CLI or tool probing canaries |
| [other/Spec.md](other/Spec.md) | Spec-driven architecture: YAML spec files as single source of truth | Read when modifying YAML specs or generator pipelines |
| [other/SharedDiskCache.md](other/SharedDiskCache.md) | Shared disk cache for local processes: TTL cache-aside + flock-guarded write | Read when implementing cross-process caching or file locks |
| [other/Website.md](other/Website.md) | Website Building Rules: hosting/branding, sibling-project inspiration, content honesty, subpage/relative-links, static/no-CDN, opt-in JS demos | Real Claude Code Agent Skill (`~/.claude/skills/website/SKILL.md`, description-matched — issue 130 item 8), no longer baked into every project's baseline AGENTS.md. `/website` slash command still works too. Copyable via `init --docs website` |
| [other/Containerfile.md](other/Containerfile.md) | Containerfile build efficiency: static-first/dynamic-last layer ordering, mandatory git/git-lfs for agent-facing images, worked example from `scripts/agent-canary/Containerfile` | Summary active in AGENTS.md. Read in full when writing or reviewing a Containerfile/Dockerfile |

**`docs/studies/`** — case studies & background reports (reference material for future generic docs)

| File | Topic |
|------|-------|
| [studies/GoRelease.md](studies/GoRelease.md) | Release pipeline case study (goreleaser consolidation) |
| [studies/2026-08-29-a-day-of-fresh-sprints.md](studies/2026-08-29-a-day-of-fresh-sprints.md) | Retrospective: a full-day `/fresh-sprint` run — 13 tickets moved, a 3-pass AGY quota bug, and what it showed about parallel-vs-sequential subagent dispatch and scope boundaries |
| [studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md](studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md) | Retrospective: `internal/uix` layout integration + watch controls overlay shipped, the AGY quota collector's live-process-only design traced through 3 stacked layers (103/104/106), and a ticket-number race that made subagent dispatch strictly sequential by default for every task type |
| [studies/2026-08-29-harnez-go-release-and-signing.md](studies/2026-08-29-harnez-go-release-and-signing.md) | Harnez Go release pipeline & non-interactive signing case study (passwordless minisign, Codeberg API `has_releases`, `--continue` recovery) |
| [studies/Worktrees.md](studies/Worktrees.md) | Worktrees case study (go.mod races, dependencies; stale-base-branch and uncommitted-state-invisibility bugs found 2026-08-28) |
| [studies/2026-08-16-harnez-migration-and-workspace-unification.md](studies/2026-08-16-harnez-migration-and-workspace-unification.md) | Harnez migration, Spec generalization & workspace diagnostics case study |
| [studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md](studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md) | Multi-agent token, session & quota monitoring case study (Claude Code, AGY, Codex) |
| [studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md](studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md) | `harnez usage --watch` TUI build + a three-theory terminal-rendering bug postmortem (stale redraw, gutter math, when to escalate to real pty/VT100 testing) |
| [studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md](studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md) | `harnez usage --summary`, silent quota-fetch failures, `--watch` stale fallback, and a flock-coordinated shared cache — including the design conversation that rejected a hand-rolled optimistic-check protocol in favor of `flock` |
| [studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md](studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md) | AI agent telemetry methods: lifecycle hooks vs. transcript seeking vs. transparent HTTP proxy sidecars across Claude, AGY, and Codex |
| [studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md](studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md) | Multi-agent lifecycle management: zombie accumulation vs. premature teardown, entity archetypes, pre-kill workspace patch preservation, and lease heartbeats |
| [studies/2026-08-19-the-5-phase-agentic-sprint-and-independent-review-loop.md](studies/2026-08-19-the-5-phase-agentic-sprint-and-independent-review-loop.md) | The 5-Phase Agentic Sprint Loop & The Independent Review Gate (Candidate chapter for *Agentic Software Development: The Ubunatic Way*) |
| [studies/2026-08-23-usage-history-subcommands-process-panel-and-remote-monitoring.md](studies/2026-08-23-usage-history-subcommands-process-panel-and-remote-monitoring.md) | Usage history subcommand decomposition, process telemetry panel (`/proc`), SSH remote monitoring (`--host` / `[r]`), and multi-host `make sync` |
| [studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md](studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md) | ADR: device telemetry (Load box CPU/GPU) is read only via kernel-standard procfs/sysfs, never a vendor's proprietary CLI/SDK — `nvidia-smi` calling was removed entirely rather than kept as a fallback |
| [studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md](studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md) | The `[L] Load` panel's kernel data sources (`/proc/stat` per-core parsing, hwmon CPU temp discovery, amdgpu sysfs attributes) and reusable patterns (burst-seeded history, decoupled redraw ticker, absolute-scale sparklines, fixed-width label columns) |
| [studies/2026-08-28-usage-collector-daemon-architecture.md](studies/2026-08-28-usage-collector-daemon-architecture.md) | Background usage-collector daemon design (issues 082–087): Omarchy Quickshell Agents widget comparison, the two-layer cache split, SQLite/DuckDB considered-and-rejected in favor of a generalized flock gate, and an offline-cache-masks-live-data bug postmortem |
| [studies/2026-08-31-harnez-tool-observability-and-the-real-environment-verification-gap.md](studies/2026-08-31-harnez-tool-observability-and-the-real-environment-verification-gap.md) | `harnez-tool-observability` (issues 115–124): eight tickets shipped with green tests while automatic capture was completely non-functional in real usage — four bugs (two found by review, two only by live-restarting a real session), and why the tests didn't catch the live-only two |

**`docs/feedback/`** — agentic retrospectives & harness feedback reports

| File | Topic |
|------|-------|
| [feedback/2026-08-18-agentic-extraction-blindspots-and-harness-gaps.md](feedback/2026-08-18-agentic-extraction-blindspots-and-harness-gaps.md) | Subagent domain extraction blindspots, wrapper traps, and proposed harnez features |
| [feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md](feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md) | Parallel advisors, sequential dev orchestration, background zombie hygiene, and pre-commit review gates |
| [feedback/2026-08-24-repo-assessment-and-managed-docs-effectiveness.md](feedback/2026-08-24-repo-assessment-and-managed-docs-effectiveness.md) | `harnez assess` delivery, token heuristics, managed docs effectiveness, and self-assessment findings |
| [feedback/2026-08-28-iterative-tui-tuning-and-empirical-verification.md](feedback/2026-08-28-iterative-tui-tuning-and-empirical-verification.md) | Iterative visual-tuning retrospective: empirical verification over guessing (taskset core-pinning test, subprocess timing), refusing to fabricate PCI ID data, reflecting back ambiguous scope, live-capturing `--watch` frames instead of trusting a build pass |
| [feedback/2026-08-28-subagent-handoff-responsiveness-and-load-memory.md](feedback/2026-08-28-subagent-handoff-responsiveness-and-load-memory.md) | Session follow-through: subagent handoff must keep the host responsive, Load panel RAM/VRAM/GTT decisions, one-line GPU memory follow-up, and remote Load telemetry cost model |
| [feedback/2026-08-29-harnez-release-engine-remote-prioritization-and-twin-quota.md](feedback/2026-08-29-harnez-release-engine-remote-prioritization-and-twin-quota.md) | Release engine & remote prioritization retrospective: passwordless minisign, Codeberg API releases enablement, URL-based priority cascade, and twin-quota alignment |
| [feedback/2026-08-29-statusline-mvp-cwd-trust-boundary-and-forge-token-fallback.md](feedback/2026-08-29-statusline-mvp-cwd-trust-boundary-and-forge-token-fallback.md) | Status-line cwd MVP retrospective: the directory-trust enforcement gap a doc convention can't close, and root-causing the `has_releases` 401 to `fj`'s `keys.json` token fallback instead of assuming env-var-only |

**`docs/proposed/`** — staging area for docs that may become copyable (no install mechanics yet).
