# Docs — Evergreen Project Docs

In-depth references for decisions, architecture, and pitfalls specific to this codebase.
Not needed for routine coding; reach for these during investigations or design work.

> [!TIP]
> **Context Discipline**: see `docs/practices/AgenticLoop.md` Invariant 6 (Context Discipline & Range-Bounded Ingestion) for the canonical rule.

| File | Topic & Consultation Trigger |
|------|------------------------------|
| [CLIDesign.md](CLIDesign.md) | apply vs init separation: scope, rationale, footgun avoided, design evolution (consult before modifying CLI command flags) |
| [CodexSettings.md](CodexSettings.md) | Codex config ownership, spec-driven debloat, status/revert, and isolation limits (consult before changing Codex settings management) |
| [CommandsPipeline.md](CommandsPipeline.md) | Claude commands and Prime prompts plus shared skills for Gemini, Codex, and Prime Agent (consult when changing command pipelines) |
| [HarnezComponents.md](HarnezComponents.md) | Component coupling analysis: subsystem map, package and runtime-contract coupling, use-case coverage, proposed separation boundaries, migration notes, and the component-selection design (§8: paths A–E, presets, removal semantics, dispatch modes, token capture; MVP in `internal/components`) (consult before splitting harnez or adding component selection) |
| [HarnezAgentArchitecture.md](HarnezAgentArchitecture.md) | `harnez agent`: command forms, prompt assembly, session attribution, roles, stream output protocol, compaction, dispatch policy (consult before changing agent dispatch or its output) |
| [OrchestratedAgentFlow.md](OrchestratedAgentFlow.md) | Orchestrator + leaf developer sprint flow: enforced roles, per-ticket loop, review checklist, pitfalls found, how to find out what agents ran (consult before an orchestrated run or when an agent misuses `harnez agent`) |
| [CodexEvents.md](CodexEvents.md) | Codex event shapes used by the analytics adapter (consult when changing Codex telemetry parsing) |
| [CodexHooks.md](CodexHooks.md) | Codex lifecycle hook reference schema (consult when changing Codex hook wiring) |
| [TokenMeasurementArchitecture.md](TokenMeasurementArchitecture.md) | Lifecycle-hook token measurement across AGY, Claude Code and Codex (consult when changing hooks or token accounting) |
| [MultimodalContextDelivery.md](MultimodalContextDelivery.md) | Context-delivery architecture and token-economics roadmap (consult when changing how docs and context reach agents) |
| [AntigravityDebloatAssessment.md](AntigravityDebloatAssessment.md) | AGY context usage and how `apply --debloat` ports to it (consult when changing AGY debloat) |
| [BrailleDot8.md](BrailleDot8.md) | Braille 8 text convention and `scripts/md-to-braille8.py` (Dot8 card experiment on hold, see issue 444) |
| [InitialRAMPAssessment.md](InitialRAMPAssessment.md) | Sibling-repository assessment of 2026-09-17 |
| [HookRewritePattern.md](HookRewritePattern.md) | Two-stage PreToolUse hook pattern (rewrite now, capture later): `<feature> hook` vs. `<feature>` wrapper, `harnez distill hook` as reference implementation, plus two hard-won constraints — hooks on the same matcher don't compose (last-to-finish wins) and a rewritten command must stay one shell token (consult before adding any new agent-hook-driven feature) |
| [LanguagePipeline.md](LanguagePipeline.md) | Language pipeline: docs install, template scaffolding, targets injection, Markers abstraction, lint |
| [MacOSPortability.md](MacOSPortability.md) | macOS/Darwin portability architecture: OS-gating, CI, dev-deps, and the standalone podmac boundary (consult before platform-specific code, CI, or guest tooling changes) |
| [MicIndicators.md](MicIndicators.md) | Linux desktop microphone privacy-indicator survey (consult when building mic-activity detection or indicators) |
| [PixelFont5x8Glyphs.md](PixelFont5x8Glyphs.md) | Design notes for the hand-tuned non-trivial 5x8 glyphs (grid rules, per-glyph tables) |
| [PixelFontArchitecture.md](PixelFontArchitecture.md) | Bitmap font pipeline in readcard: single glyph spec, embedded upstream BDFs, lazy loading, goldens, lessons learned |
| [Models.md](Models.md) | Model assessment: 2026-09-23 web-research snapshot per configured model (Go, TUI, agent tooling, CLI, cost) plus earlier comparison notes; operational guidance is `harnez agent models` (consult when picking a model tier for a subagent or workflow) |
| [ModelAdvisoryEval.md](ModelAdvisoryEval.md) | Five-model advisory eval (luna:low/med, terra:low, haiku, sonnet) on one sprint plan: fact checks against the repo, cost, judgment, role takeaways, 491 sprint data; generic copyable version: `practices/ModelRoles.md` |
| [Permissions.md](Permissions.md) | Claude Code permission model; Bash vs Read layers; grow-only caveat (consult when updating permission schemas) |
| [Roadmap.md](Roadmap.md) | Working roadmap synthesized from the open issue backlog (consult before prioritizing new work; regenerate via `/roadmap`) |
| [Telemetry.md](Telemetry.md) | Telemetry and evolution architecture: multi-agent usage tracking, quota history snapshotting, multi-track Git artifact evolution, and project token attribution, and the store (serialized migration, spec-defined SQL, `stats --quality`) (consult when modifying analytics or usage pipelines) |
| [TUIDesign.md](TUIDesign.md) | Single-cell indicator semantics, panel visibility model invariants, and raw-mode hotkey contracts (consult when changing gauges, multi-panel layouts, or key dispatch) |
| [Bench.md](Bench.md) | Optional `harnez bench` harness: specced agent tasks under lite/full docs and PNG cards, own DB | Read when benchmarking doc delivery across agents |
| [Testing.md](Testing.md) | Test layers and verification entry points for package, integration, static, smoke, canary, and live checks |


Copyable docs (installed to Claude and Prime Agent global dirs on `apply`, copied to projects via `--docs`) live in subdirs.
Generated copies land here (root) after `apply` / `init`.

**`docs/lang/`** — language/SDK/framework docs *(High-level rule summaries are already bundled in AGENTS.md; read files only for detailed templates or syntax nuances)*

| File | Topic | Read Trigger / Scope |
|------|-------|----------------------|
| [lang/Go.md](lang/Go.md) | Go conventions | Bounded read only for Cobra flag idioms, error wrapping conventions, or test templates |
| [lang/ManPages.md](lang/ManPages.md) | Man pages for Go/Cobra CLIs | Read when wiring `<cmd> man`/`--install`, `cobra/doc` generation, or GoReleaser/NFPM man page packaging |
| [lang/Bash.md](lang/Bash.md) | Bash/Shell conventions | Read for exact `if test` construct syntax, formatting rules, or directory-flag scoping (`git -C`/`make -C` over `cd`) |
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

This table is regenerated by `harnez index` (issue 148) from `docs/studies/*.md`, in
filename order. The Topic column comes from, in priority order: an explicit
`<!-- harnez:topic: ... -->` comment in the study file, else its `**Scope**:` field
(joined across wrapped lines), else its H1 title with common prefixes stripped. Add a
`harnez:topic` comment to a study when the derived text is too long or generic for the
table — don't hand-edit the row here, it will be overwritten on the next run.

| File | Topic |
|------|-------|
| [studies/2026-08-16-harnez-migration-and-workspace-unification.md](studies/2026-08-16-harnez-migration-and-workspace-unification.md) | Spec Generalization, Harnez Migration & Workspace Diagnostics |
| [studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md](studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md) | Token, session, and rate-limit tracking across Claude Code, Antigravity (AGY), and OpenAI Codex CLI |
| [studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md](studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md) | `harnez usage --summary`, three robustness fixes to live quota fetching in `internal/usage/` |
| [studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md](studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md) | Live-refreshing btop-style dashboard for `harnez usage`, and the debugging saga that followed |
| [studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md](studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md) | Telemetry, token tracking, and quota monitoring across Claude Code, Google Antigravity (AGY), and OpenAI Codex CLI |
| [studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md](studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md) | Multi-agent orchestration, subagent lifecycle state machines, workspace preservation, and process teardown hygiene across Claude Code and Google Antigravity (AGY) |
| [studies/2026-08-19-the-5-phase-agentic-sprint-and-independent-review-loop.md](studies/2026-08-19-the-5-phase-agentic-sprint-and-independent-review-loop.md) | The 5-Phase Agentic Sprint Loop & The Independent Review Gate |
| [studies/2026-08-23-usage-history-subcommands-process-panel-and-remote-monitoring.md](studies/2026-08-23-usage-history-subcommands-process-panel-and-remote-monitoring.md) | `cmd/harnez/main.go`, `internal/usage/`, `Makefile`, issue tickets 048–051 |
| [studies/2026-08-26-cross-repo-managed-docs-and-agentic-tooling-design.md](studies/2026-08-26-cross-repo-managed-docs-and-agentic-tooling-design.md) | Cross-Repo Managed Docs, Interactive Grounding, and Agent-First Tooling Design |
| [studies/2026-08-28-fresh-sprint-rograph-three-ticket-token-study.md](studies/2026-08-28-fresh-sprint-rograph-three-ticket-token-study.md) | Empirical token/tool accounting for a single `/fresh-sprint` run executing three dependency-ordered tickets (078 → 079 → 080) via sequential subagent handoffs, plus a modeled counterfactual for running the same work in one continuous main-session context. |
| [studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md](studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md) | `internal/usage/load.go` (Load box CPU/GPU collectors); applies to any future device-metrics collector |
| [studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md](studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md) | `internal/usage/load.go`, `internal/usage/watch.go` |
| [studies/2026-08-28-usage-collector-daemon-architecture.md](studies/2026-08-28-usage-collector-daemon-architecture.md) | Usage-Collector Daemon Architecture, Omarchy Comparison, and Rejected Alternatives |
| [studies/2026-08-29-a-day-of-fresh-sprints.md](studies/2026-08-29-a-day-of-fresh-sprints.md) | A Day of Fresh Sprints |
| [studies/2026-08-29-harnez-go-release-and-signing.md](studies/2026-08-29-harnez-go-release-and-signing.md) | Harnez Go Release Pipeline & Non-Interactive Signing Case Study |
| [studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md](studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md) | Usage Panel Integration and the AGY Collector Cliff |
| [studies/2026-08-31-agy-clean-session-system-prompt-audit.md](studies/2026-08-31-agy-clean-session-system-prompt-audit.md) | Same clean-session method repeated for agy (Antigravity); repetition/prunability findings only — includes the caveat that agy currently runs on Claude models underneath, so this is a harness/prompt-engineering comparison, not a cross-model one |
| [studies/2026-08-31-bar-rendering-visual-debugging-and-spec-bootstrap.md](studies/2026-08-31-bar-rendering-visual-debugging-and-spec-bootstrap.md) | Bar-Rendering Visual Debugging, the `spec/` Bootstrap, and the `rograph` Boundary |
| [studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md](studies/2026-08-31-claude-code-clean-session-system-prompt-audit.md) | Clean-session self-audit method (`claude -p` / `claude -c -p` in an empty scratch dir): Claude Code's own harness-native system prompt measured for repetition and prunability, and why the Tool Feedback Protocol directive gets skipped despite being correctly injected |
| [studies/2026-08-31-harnez-tool-observability-and-the-real-environment-verification-gap.md](studies/2026-08-31-harnez-tool-observability-and-the-real-environment-verification-gap.md) | `harnez-tool-observability` (issues 115–124): eight tickets shipped with green tests while automatic capture was completely non-functional in real usage — four bugs (two found by review, two only by live-restarting a real session), and why the tests didn't catch the live-only two |
| [studies/2026-08-31-instruction-distribution-audit-synthesis.md](studies/2026-08-31-instruction-distribution-audit-synthesis.md) | Product-discovery + technical-advisor synthesis on global/user-home vs. per-project instruction distribution and duplication; prioritized findings table that became [[130]]'s 8-item scope |
| [studies/2026-09-01-usage-watch-startup-splash-and-usage-flag-redesign.md](studies/2026-09-01-usage-watch-startup-splash-and-usage-flag-redesign.md) | issues 164–172; `internal/usage/watch.go`, `internal/usage/usage.go`, `internal/usage/agy.go`, `cmd/harnez/main.go` |
| [studies/2026-09-02-git-history-telemetry-release-generics-and-multi-agent-ergonomics.md](studies/2026-09-02-git-history-telemetry-release-generics-and-multi-agent-ergonomics.md) | Git history telemetry (`harnez dochistory`), WebExtension release generics, load box timeseries alignment, and multi-agent issue reservation |
| [studies/2026-09-03-cross-project-feedback-promotion-and-privacy-level-exports.md](studies/2026-09-03-cross-project-feedback-promotion-and-privacy-level-exports.md) | Cross-project feedback promotion (issues 202/203) and the 4-level privacy-scrubbed usage/telemetry export pipeline (issue 204), plus a second case of independent parallel-session convergence |
| [studies/2026-09-03-taxonomic-telemetry-classification-and-terminal-layout-invariants.md](studies/2026-09-03-taxonomic-telemetry-classification-and-terminal-layout-invariants.md) | Taxonomic telemetry note classification (issue 212), ergonomic issue discovery (issue 217), and visual layout invariant enforcement (issue 218) |
| [studies/2026-09-04-fleet-wide-24h-cross-project-survey.md](studies/2026-09-04-fleet-wide-24h-cross-project-survey.md) | Fleet-wide 24h survey (2026-09-03→04) across harnez, lmcoder, ubunatic.com, voxi, uman, psync — 95 commits, ~15k line delta, six independently-driven sprints observed from git history rather than a single session |
| [studies/2026-09-05-issues-verb-roadmap-discovery-and-index-data-loss.md](studies/2026-09-05-issues-verb-roadmap-discovery-and-index-data-loss.md) | `cmd/harnez/issues.go`, `internal/issues/issues.go`, `internal/index/index.go`, `commands/issue.md`, `commands/roadmap.md`, `commands/discovery.md`, tickets 232–240. |
| [studies/2026-09-08-cross-repo-release-onboarding-and-the-gowork-silent-substitution-trap.md](studies/2026-09-08-cross-repo-release-onboarding-and-the-gowork-silent-substitution-trap.md) | Cross-Repo Release Onboarding and the go.work Silent-Substitution Trap |
| [studies/2026-09-09-external-agent-review-and-dispatch.md](studies/2026-09-09-external-agent-review-and-dispatch.md) | External Agent Review, Artifact-First Completion, and Claude Dispatch Lessons |
| [studies/2026-09-10-advisor-session-reuse-and-cross-agent-dispatch.md](studies/2026-09-10-advisor-session-reuse-and-cross-agent-dispatch.md) | Advisor session reuse, cache evidence, and cross-agent dispatch design |
| [studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md](studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md) | Pi, DeepSeek Harness, and Prime Agent plugin systems and self-modification |
| [studies/2026-09-11-first-live-codex-advisor-design-gate-and-a-third-init-drop-recurrence.md](studies/2026-09-11-first-live-codex-advisor-design-gate-and-a-third-init-drop-recurrence.md) | First live harnez-advisor Codex design-review call, and a third live recurrence of the opt-in-doc-drop bug |
| [studies/2026-09-13-cli-invocation-log-exit-code-anti-pattern-and-self-pollution.md](studies/2026-09-13-cli-invocation-log-exit-code-anti-pattern-and-self-pollution.md) | harnez log feature arc (cli_invocations, attribution, log verb), the os.Exit-in-RunE anti-pattern it exposed, and a telemetry self-pollution bug found while dogfooding it |
| [studies/2026-09-15-claude-code-auto-memory-assessment.md](studies/2026-09-15-claude-code-auto-memory-assessment.md) | Claude Code Auto-Memory: Content Assessment & Cleanup Options |
| [studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md](studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md) | measured token-context impact of the settings.json debloat feature (issue 316) presets against claude -p "/context", and how that measurement axis compares to the orthogonal --bare and --safe-mode CLI flags |
| [studies/2026-09-15-websearch-vs-webfetch-comparison.md](studies/2026-09-15-websearch-vs-webfetch-comparison.md) | side-by-side comparison of Claude Code's WebSearch and WebFetch tools via two isolated subagents fetching/searching the same real target (harnez.org/tools/voxi), covering accuracy, token cost, and when each is the right choice |
| [studies/2026-09-16-agent-token-and-quota-trackability-status.md](studies/2026-09-16-agent-token-and-quota-trackability-status.md) | Token accounting, per-step / tool usage breakdown, full session lifecycle metrics, and weekly subscription quota tracking across Google Antigravity (AGY), Anthropic Claude Code, and OpenAI Codex CLI. |
| [studies/2026-09-16-fleet-usage-and-token-history-analysis.md](studies/2026-09-16-fleet-usage-and-token-history-analysis.md) | 100-day fleet usage and token timeseries analysis across x600, t14, um760, and multi-agent harnesses |
| [studies/2026-09-16-quota-1-guardrail-architecture-and-multi-agent-concurrency.md](studies/2026-09-16-quota-1-guardrail-architecture-and-multi-agent-concurrency.md) | Quota-1 Guardrail Architecture, Multi-Agent Concurrency, and Lite Doc Optimization |
| [studies/2026-09-16-ramp-levels-and-go-repository-init.md](studies/2026-09-16-ramp-levels-and-go-repository-init.md) | Go libraries and Go CLI applications; TUI support in the MVP, systemd services as a later extension |
| [studies/2026-09-16-skilloverrides-live-verification.md](studies/2026-09-16-skilloverrides-live-verification.md) | live verification of Claude Code's skillOverrides settings.json field against an installed version, resolving issue 347's unverified GitHub-issue claims |
| [studies/2026-09-17-agenticloop-canary-and-global-doc-deactivation.md](studies/2026-09-17-agenticloop-canary-and-global-doc-deactivation.md) | AgenticLoop Real-Invocation Canary, and Global Doc Deactivation |
| [studies/2026-09-17-doc-screenshots-and-vision-token-efficiency-benchmark.md](studies/2026-09-17-doc-screenshots-and-vision-token-efficiency-benchmark.md) | Claude 3.7 (`claude`), Antigravity / Gemini (`agy`), OpenAI / Codex |
| [studies/2026-09-17-eager-global-doc-include-stripping-and-startup-token-tax.md](studies/2026-09-17-eager-global-doc-include-stripping-and-startup-token-tax.md) | Eager Global Doc Include Stripping, Startup Token Tax, and Prompt Distribution Architecture |
| [studies/2026-09-17-multi-agent-issue-specification-benchmarking-and-remote-inference.md](studies/2026-09-17-multi-agent-issue-specification-benchmarking-and-remote-inference.md) | Multi-Agent Issue Specification Benchmarking & Remote Inference Architecture |
| [studies/2026-09-17-one-shotting-advanced-features-multimodal-context-delivery.md](studies/2026-09-17-one-shotting-advanced-features-multimodal-context-delivery.md) | Case study on rapid, zero-regression one-shot development of multimodal context delivery, ViT benchmarks, canary verification, and retro pixel font engine |
| [studies/2026-09-17-retro-pixel-fonts-and-micro-vit-compression-study.md](studies/2026-09-17-retro-pixel-fonts-and-micro-vit-compression-study.md) | Claude 3.7 (`claude`), Gemini 2.0 / Flash (`agy`), OpenAI / Codex (GPT-4o/o1/o3) |
| [studies/2026-09-17-visual-issue-discovery-and-multimodal-agent-trajectories.md](studies/2026-09-17-visual-issue-discovery-and-multimodal-agent-trajectories.md) | Case study on multimodal issue discovery, token compression via visual cards, and autonomous subagent trajectory verification |
| [studies/2026-09-18-codex-multimodal-context-reading-and-visual-card-economics.md](studies/2026-09-18-codex-multimodal-context-reading-and-visual-card-economics.md) | Codex Multimodal Context Reading, Visual Cards, and Telemetry Economics |
| [studies/2026-09-20-braille-versus-direct-document-reader-token-study.md](studies/2026-09-20-braille-versus-direct-document-reader-token-study.md) | Braille-mediated versus direct-Markdown subagent reading cost |
| [studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md](studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md) | Dot8 Braille vs. Markdown and Multimodal Context Card Token Benchmarks |
| [studies/2026-09-20-dot8-larger-dot-card-canary.md](studies/2026-09-20-dot8-larger-dot-card-canary.md) | Dot8 Larger-Dot Card Canary (2026-09-20) |
| [studies/2026-09-20-haiku-dot8-card-reading-canary.md](studies/2026-09-20-haiku-dot8-card-reading-canary.md) | Haiku Dot8 card reading canary (2026-09-20) |
| [studies/CrossHarnessSubagentReport.md](studies/CrossHarnessSubagentReport.md) | Cross-Harness Subagent Dispatch Benchmark: Codex (Luna) vs. Claude Code (Haiku) |
| [studies/GoRelease.md](studies/GoRelease.md) | Go Release Pipeline Proposal |
| [studies/MacOSContainerAMD.md](studies/MacOSContainerAMD.md) | macOS Containers on AMD KVM & Podman: Architecture, Quirks, and Diagnostic Guide |
| [studies/Quota1Approach.md](studies/Quota1Approach.md) | Research Study: Quota-1 Guardrails for LLM Agent Loops |
| [studies/RTKShellWrapperHandling.md](studies/RTKShellWrapperHandling.md) | RTK: Shell-Wrapper and Pipe Handling in Command-Rewrite Hooks |
| [studies/Worktrees.md](studies/Worktrees.md) | Worktrees — learnings & TODOs |

**`docs/feedback/`** — agentic retrospectives & harness feedback reports

| File | Topic |
|------|-------|
| [feedback/2026-09-15-podmac-extraction-and-live-verification.md](feedback/2026-09-15-podmac-extraction-and-live-verification.md) | Podmac extraction, live favicon verification, storage/mount boundaries, and missed physical-directory cleanup |
| [feedback/2026-08-18-agentic-extraction-blindspots-and-harness-gaps.md](feedback/2026-08-18-agentic-extraction-blindspots-and-harness-gaps.md) | Subagent domain extraction blindspots, wrapper traps, and proposed harnez features |
| [feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md](feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md) | Parallel advisors, sequential dev orchestration, background zombie hygiene, and pre-commit review gates |
| [feedback/2026-08-24-repo-assessment-and-managed-docs-effectiveness.md](feedback/2026-08-24-repo-assessment-and-managed-docs-effectiveness.md) | `harnez assess` delivery, token heuristics, managed docs effectiveness, and self-assessment findings |
| [feedback/2026-08-28-iterative-tui-tuning-and-empirical-verification.md](feedback/2026-08-28-iterative-tui-tuning-and-empirical-verification.md) | Iterative visual-tuning retrospective: empirical verification over guessing (taskset core-pinning test, subprocess timing), refusing to fabricate PCI ID data, reflecting back ambiguous scope, live-capturing `--watch` frames instead of trusting a build pass |
| [feedback/2026-08-28-subagent-handoff-responsiveness-and-load-memory.md](feedback/2026-08-28-subagent-handoff-responsiveness-and-load-memory.md) | Session follow-through: subagent handoff must keep the host responsive, Load panel RAM/VRAM/GTT decisions, one-line GPU memory follow-up, and remote Load telemetry cost model |
| [feedback/2026-08-29-harnez-release-engine-remote-prioritization-and-twin-quota.md](feedback/2026-08-29-harnez-release-engine-remote-prioritization-and-twin-quota.md) | Release engine & remote prioritization retrospective: passwordless minisign, Codeberg API releases enablement, URL-based priority cascade, and twin-quota alignment |
| [feedback/2026-08-29-statusline-mvp-cwd-trust-boundary-and-forge-token-fallback.md](feedback/2026-08-29-statusline-mvp-cwd-trust-boundary-and-forge-token-fallback.md) | Status-line cwd MVP retrospective: the directory-trust enforcement gap a doc convention can't close, and root-causing the `has_releases` 401 to `fj`'s `keys.json` token fallback instead of assuming env-var-only |
| [feedback/2026-09-01-time-gauge-ansi-padding-review.md](feedback/2026-09-01-time-gauge-ansi-padding-review.md) | Time-gauge ANSI styling retrospective: apply colors after label padding and test both raw SGR sequences and stripped geometry |
| [feedback/2026-09-22-component-system-design-doc-first.md](feedback/2026-09-22-component-system-design-doc-first.md) | Component system retro (489/490): doc before code for design tickets, MVP in a new package, removal passes expose ownership bugs (Codex hooks), session-env-dependent tests |
| [feedback/2026-09-22-lean-sprint-496-488-497.md](feedback/2026-09-22-lean-sprint-496-488-497.md) | Retro: luna:low lean sprint (496, 488, 497): assertion-hidden bug, interface change needs a higher tier, quota cost measured |

**`docs/proposed/`** — staging area for docs that may become copyable (no install mechanics yet).
