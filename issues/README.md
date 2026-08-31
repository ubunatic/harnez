# Issues

Concrete bugs, design issues, and feature gaps. Each file has a status line.

Archived (resolved/closed) issues are in `issues/archive/`.

| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [archive/001-diff-clean-wrong-path.md](archive/001-diff-clean-wrong-path.md) | diff and clean operate on wrong file when --project is set | Closed — resolved |
| 002 | [002-lang-symlink-no-expandhome.md](002-lang-symlink-no-expandhome.md) | lang symlink doesn't expand ~ in target | Closed — invalid |
| 003 | [003-mergedocs-dedup-bug.md](003-mergedocs-dedup-bug.md) | mergeDocs dedup bug (renamed from mergeLangs) | Closed |
| 004 | [004-diff-exit-code-swallowed.md](004-diff-exit-code-swallowed.md) | diff exit code swallowed | Closed |
| 005 | [005-permissions-grow-only.md](005-permissions-grow-only.md) | permissions are grow-only; revoked entries never removed | Open |
| 006 | [006-status-checks-only-model.md](006-status-checks-only-model.md) | status settings.json check covers only model key | Open — partially fixed |
| 007 | [007-no-tests.md](007-no-tests.md) | thin test coverage — gaps remaining | Open — partially addressed |
| 008 | [archive/008-commands-summary-excludes-new.md](archive/008-commands-summary-excludes-new.md) | Apply summary omits newly written commands | Closed — fixed |
| 009 | [009-diff-clean-no-makefile-targets.md](009-diff-clean-no-makefile-targets.md) | diff and clean don't cover Makefile targets section | Open |
| 010 | [010-smoke-test-agent-visibility.md](010-smoke-test-agent-visibility.md) | smoke-test that agents can see installed skills and commands | Open — blocked on wayreel#11 |
| 011 | [011-autodetect-nondeterministic-order.md](011-autodetect-nondeterministic-order.md) | autoDetectDocs non-deterministic order causes spurious AGENTS.md diffs | Closed |
| 012 | [012-website-feature-demo-video.md](012-website-feature-demo-video.md) | website: feature the TUI demo video + favicon | Open — blocked on clean recording & user verification |
| 013 | [013-promote-command.md](013-promote-command.md) | promote command — push improved project docs back to source | Open |
| 014 | [014-zig-language-doc.md](014-zig-language-doc.md) | bundle a Zig language doc (emojig, books need it) | Closed |
| 015 | [015-agents-md-uman-workspace-awareness.md](015-agents-md-uman-workspace-awareness.md) | AGENTS.md must teach agents about uman (workspace glue) | Open |
| 016 | [016-make-smoke-convention.md](016-make-smoke-convention.md) | `make smoke` convention: live-run targets in Make.md | Open |
| 017 | [017-spec-md-is-cati-specific-not-generic.md](017-spec-md-is-cati-specific-not-generic.md) | `docs/Spec.md` (bundled, default: true) is Cati-specific, not a generic guide | Closed |
| 018 | [018-mark-bundled-docs-in-frontmatter.md](018-mark-bundled-docs-in-frontmatter.md) | Bundled docs don't self-identify as claudeconfig-managed | Open |
| 019 | [019-rename-to-harnez.md](019-rename-to-harnez.md) | Rename project from claudeconfig to harnez | Closed |
| 020 | [archive/020-tools-command-os-tools.md](archive/020-tools-command-os-tools.md) | `harnez tools`: guided OS-level tool installation | Closed — extracted to `voxi` |
| 023 | [archive/023-usage-command-token-quota-tracking.md](archive/023-usage-command-token-quota-tracking.md) | `harnez usage`: unified token, session & quota status command | Closed — resolved in `48a2585`, `6c82176` |
| 029 | [archive/029-extract-ubunatic-voxi-standalone.md](archive/029-extract-ubunatic-voxi-standalone.md) | Extract standalone voice input engine (`ubunatic/voxi`) | Complete — extracted to standalone repo |
| 030 | [030-agy-codex-missing-local-token-counts.md](030-agy-codex-missing-local-token-counts.md) | AGY and Codex have no local token-count source | Open |
| 031 | [031-usage-quota-fetch-errors-silent.md](031-usage-quota-fetch-errors-silent.md) | Live quota fetch failures are silent in `harnez usage` | Closed |
| 032 | [032-usage-watch-no-stale-fallback-on-fetch-failure.md](032-usage-watch-no-stale-fallback-on-fetch-failure.md) | `--watch` blanks quota panels instead of keeping last-known-good data on a transient fetch failure | Closed |
| 033 | [033-usage-shared-quota-cache.md](033-usage-shared-quota-cache.md) | Shared disk-backed quota cache to stop concurrent `harnez` instances from double-polling | Closed |
| 034 | [034-hook-triggered-token-extraction.md](034-hook-triggered-token-extraction.md) | Hook-triggered file seek for local token extraction (AGY & Claude) | Open |
| 035 | [035-transparent-proxy-quota-and-token-sidecar.md](035-transparent-proxy-quota-and-token-sidecar.md) | Transparent local HTTP_PROXY sidecar for zero-wrap rate limit & quota interception | Open |
| 036 | [036-harnez-status-issues-tracker-linter.md](036-harnez-status-issues-tracker-linter.md) | `harnez status`: issues tracker status linter & reconciliation | Closed |
| 037 | [037-harnez-diff-exit-code-flag.md](037-harnez-diff-exit-code-flag.md) | `harnez diff --exit-code`: return status 1 on detected drift for CI | Closed |
| 038 | [038-research-subagent-lifecycle-and-cleanup-friction.md](038-research-subagent-lifecycle-and-cleanup-friction.md) | Research: automated subagent lifecycle hooks & teardown friction | Closed (Research Complete) |
| 039 | [039-agentic-loop-practices-and-sprint-command.md](039-agentic-loop-practices-and-sprint-command.md) | `docs/practices/AgenticLoop.md` and `/sprint` command scaffolding | Closed |
| 040 | [040-agent-context-duplication-and-file-read-discipline.md](040-agent-context-duplication-and-file-read-discipline.md) | Agent Context Ingestion Duplication & File Read Discipline | Closed |
| 041 | [041-sibling-projects-real-world-shape-and-test-suite-extensions.md](041-sibling-projects-real-world-shape-and-test-suite-extensions.md) | Sibling Projects Audit: Real-World Shapes, Drift Causes & Test Suite Extensions | Closed |
| 042 | [042-agentic-loop-repro-before-fix-and-single-status-field.md](042-agentic-loop-repro-before-fix-and-single-status-field.md) | `docs/practices/AgenticLoop.md` is missing "repro before fix" and "one Status field per ticket" guidance | Open |
| 043 | [043-never-blindly-revert-commit-stale-work-first.md](043-never-blindly-revert-commit-stale-work-first.md) | Never blindly `git revert`/`checkout --`/`stash drop` unsuccessful work; commit stale/failed code first | Closed |
| 044 | [044-git-md-proactive-commit-vs-harness-ask-first.md](044-git-md-proactive-commit-vs-harness-ask-first.md) | `docs/Git.md`'s "commit proactively" conflicts with harness ask-first defaults | Closed — resolved in docs |
| 045 | [045-review-loops-harden-symptoms-not-root-cause.md](045-review-loops-harden-symptoms-not-root-cause.md) | Multi-round review loops harden the symptom, not the root cause | Open |
| 046 | [046-commit-checkpoint-recurred-after-044-filed.md](046-commit-checkpoint-recurred-after-044-filed.md) | The exact gap from 044 recurred in the same session that filed it | Open |
| 047 | [047-sparkline-vis-model-usage-over-time.md](047-sparkline-vis-model-usage-over-time.md) | Add sparkline visualization to show model usage over time | Closed — resolved |
| 048 | [048-usage-history-subcommands-refactor.md](048-usage-history-subcommands-refactor.md) | Refactor usage history flags into subcommands | Closed — resolved |
| 049 | [049-running-agent-processes-watch-panel.md](049-running-agent-processes-watch-panel.md) | Add running agent processes status box in usage watch/summary | Closed — resolved |
| 050 | [050-remote-host-flag-and-watch-hotkey.md](050-remote-host-flag-and-watch-hotkey.md) | Support remote host query via `--host` flag and `[r]` hotkey in watch | Closed — resolved |
| 051 | [051-multi-host-remote-monitoring-and-dashboard-navigation.md](051-multi-host-remote-monitoring-and-dashboard-navigation.md) | Multi-host remote usage monitoring & interactive host navigation | Open |
| 052 | [052-headless-agent-cli-probes-for-idle-telemetry-refresh.md](052-headless-agent-cli-probes-for-idle-telemetry-refresh.md) | Research: Headless CLI status probes to refresh telemetry when agents are idle | Closed — research complete |
| 053 | [053-stale-lsp-diagnostics-noise-detect-and-toggle.md](053-stale-lsp-diagnostics-noise-detect-and-toggle.md) | Background LSP diagnostics post stale/wrong findings; harness should detect and offer to disable per-agent | Open |
| 054 | [archive/054-commit-filed-issues-immediately.md](archive/054-commit-filed-issues-immediately.md) | Filed issue-tracker files should be committed immediately, not batched | Closed — resolved in a86ef7d |
| 055 | [055-no-long-sleep-use-scheduled-wakeups.md](055-no-long-sleep-use-scheduled-wakeups.md) | Agents must not use long `sleep` to wait; schedule a wakeup/BG task instead | Closed — resolved |
| 056 | [056-agenticloop-buffered-long-running-output-antipattern.md](056-agenticloop-buffered-long-running-output-antipattern.md) | AgenticLoop.md anti-patterns: add "piping long-running output through a buffering filter" | Open |
| 057 | [057-repo-assessment-and-code-metrics-command.md](057-repo-assessment-and-code-metrics-command.md) | `harnez assess`: Fast Code/Doc Metrics & Repo Feasibility Report | Closed — resolved |
| 058 | [058-deployment-transparency-and-concise-pairing-mode.md](058-deployment-transparency-and-concise-pairing-mode.md) | Deployment Transparency, Live State Grounding, and Concise Pairing Mode | Closed — resolved 2026-08-29 |
| 059 | [archive/059-capture-managed-docs-drift-to-inbox.md](archive/059-capture-managed-docs-drift-to-inbox.md) | Capture Managed Docs Drift to Inbox Markdown | Closed — resolved in 32218cb |
| 060 | [archive/060-triage-sibling-managed-docs-drift.md](archive/060-triage-sibling-managed-docs-drift.md) | Triage Sibling Managed Docs Drift Captured from Inbox Sweep | Closed — resolved |
| 061 | [061-containerfile-guidelines-for-fast-incremental-builds.md](061-containerfile-guidelines-for-fast-incremental-builds.md) | Containerfile.md: Guidelines for Fast, Cached, and Incremental Container Builds | Closed — resolved |
| 062 | [archive/062-scan-docs-across-agent-projects.md](archive/062-scan-docs-across-agent-projects.md) | `harnez scan-docs`: Combined Managed-Docs Scan Across Agent Projects | Closed — resolved |
| 063 | [archive/063-subagent-model-selection-guidance.md](archive/063-subagent-model-selection-guidance.md) | Document Fast-Capable Subagent Model Selection | Closed — resolved in docs |
| 064 | [archive/064-fresh-handoff-workflow-skill-and-friction-reporting.md](archive/064-fresh-handoff-workflow-skill-and-friction-reporting.md) | Lean Fresh-Handoff Workflow Skill and Calibrated Friction Reporting | Closed — resolved |
| 065 | [065-concisemode-caveman-skill-and-output-distillation.md](065-concisemode-caveman-skill-and-output-distillation.md) | ConciseMode (Caveman Skill) & Command Output Distillation Hook (`harnez distill`) | Closed — resolved |
| 066 | [066-native-go-command-output-distillation.md](066-native-go-command-output-distillation.md) | Native Go Command Output Distillation (`harnez distill`) Architecture | Closed — resolved |
| 067 | [067-no-mermaid-in-plain-chat-use-ascii-box-art.md](067-no-mermaid-in-plain-chat-use-ascii-box-art.md) | Do Not Render Mermaid in Plain Chat; Use ASCII Box/Arrow Art (Mermaid Reserved for Docs Only) | Closed — resolved |
| 068 | [068-harnez-sync-autonomous-doc-reconciliation-command.md](068-harnez-sync-autonomous-doc-reconciliation-command.md) | `/harnez-sync`: Autonomous Multi-Repo Managed-Docs Discovery & Auto-Reconciliation | Closed — resolved |
| 069 | [069-pretooluse-hook-optional-autopipe-bash-through-distill.md](069-pretooluse-hook-optional-autopipe-bash-through-distill.md) | PreToolUse Hook: Optional Auto-Pipe of Noisy Bash Commands Through `harnez distill` | Closed — resolved |
| 070 | [070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode.md](070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode.md) | Cross-Agent `distill` Auto-Pipe Hook: AGY, Codex, Pi, OpenCode | Closed — resolved with deterministic Pi/OpenCode hook canaries and local-model liveness evidence |
| 071 | [071-agent-canary-container-for-hook-testing.md](071-agent-canary-container-for-hook-testing.md) | Agent Canary Architecture: Separate `harnez` Container, `lmcoder` Hosts Server+Proxy Only | Open — architecture decided |
| 072 | [072-agent-canary-local-llm-pi-opencode.md](072-agent-canary-local-llm-pi-opencode.md) | Agent Canary Container: Pi + OpenCode Against Local `lmcoder` Backend | Closed — live local Pi/OpenCode liveness verified; hook-fire probe inconclusive, tracked by [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]] |
| 073 | [073-agent-canary-cloud-credentialed-claude-agy-codex.md](073-agent-canary-cloud-credentialed-claude-agy-codex.md) | Agent Canary Container: Claude/AGY/Codex With Mounted Credentials (Deferred) | Open — deferred |
| 074 | [074-distill-hook-bypassed-by-shell-wrapper-prefixes.md](074-distill-hook-bypassed-by-shell-wrapper-prefixes.md) | `harnez distill hook`'s Command Match Is Bypassed by Shell-Wrapper Prefixes | Closed — won't fix |
| 075 | [075-concisemode-promote-doc-to-real-skill.md](075-concisemode-promote-doc-to-real-skill.md) | ConciseMode: Dynamic Runtime Switch & AGENTS.md Synchronization (`harnez mode`) | Closed — resolved |
| 076 | [076-agents-local-overlay-for-runtime-mode-switching.md](076-agents-local-overlay-for-runtime-mode-switching.md) | `AGENTS.local.md` Overlay & Ephemeral Runtime Mode Switching | Closed — resolved |
| 077 | [077-box-diagram-width-and-right-margin-rules.md](077-box-diagram-width-and-right-margin-rules.md) | Box Diagram Width Limits & Right-Side Padding Rules (Prevent Terminal Line Wrap Collapse) | Closed — resolved in `docs/lang/Markdown.md` |
| 078 | [078-rograph-library-shared-bar-sparkline-renderer.md](078-rograph-library-shared-bar-sparkline-renderer.md) | `rograph`: Shared Read-Only Graph Library for Usage Bars & Sparklines | Closed — resolved in aa06ad7 |
| 079 | [079-consistent-10-char-shrink-to-fit-graphs.md](079-consistent-10-char-shrink-to-fit-graphs.md) | Apply Consistent Max-10-Char Shrink-to-Fit Width to All Usage Graphs | Closed — resolved in 9914010 |
| 080 | [080-agent-box-layout-extraction.md](080-agent-box-layout-extraction.md) | Extract Agent-Box Content Layout Into a Small Layout Library | Closed — resolved in 788e1bd |
| 081 | [081-table-hyperlink-label-width-vs-flowing-text-full-paths.md](081-table-hyperlink-label-width-vs-flowing-text-full-paths.md) | Table Hyperlink Labels Break Layout; Flowing Text Should Keep Full Paths | Closed — resolved in `docs/lang/Markdown.md` |
| 082 | [082-agent-usage-collector-daemon.md](082-agent-usage-collector-daemon.md) | Background Usage-Collector Daemon (systemd user service) | Closed — resolved |
| 083 | [083-usage-tui-self-hiding-auto-discovery.md](083-usage-tui-self-hiding-auto-discovery.md) | Self-Hiding, Auto-Discovery Agent Display in Usage TUI | Closed — resolved (uncommitted, pending transplant) |
| 084 | [084-aggregate-quota-window-box-assessment.md](084-aggregate-quota-window-box-assessment.md) | Aggregate Quota-Window Box: UX & Feasibility Assessment | Open — deferred, needs assessment before implementation |
| 085 | [085-watch-tui-show-collector-daemon-status.md](085-watch-tui-show-collector-daemon-status.md) | Briefly Show Agent-Collector Daemon Status in `harnez usage --watch` | Open |
| 086 | [archive/086-offline-degraded-cache-snapshot-masks-live-data.md](archive/086-offline-degraded-cache-snapshot-masks-live-data.md) | Offline/Degraded Collector Snapshot Is Cached and Served as Fresh, Masking Richer Live Data | Closed — resolved in `4a4b9ab` |
| 087 | [archive/087-generalize-flock-freshness-gate-to-codex-agy.md](archive/087-generalize-flock-freshness-gate-to-codex-agy.md) | Generalize Claude's flock+Freshness Live-Fetch Gate to Codex and AGY | Closed — resolved in `558dd45` |
| 088 | [088-load-panel-ram-vram-gtt-memory.md](088-load-panel-ram-vram-gtt-memory.md) | Load Panel: RAM, VRAM, and GTT Memory | Closed — resolved in implementation commit |
| 089 | [089-load-panel-combine-gpu-vram-gtt-row.md](089-load-panel-combine-gpu-vram-gtt-row.md) | Load Panel: Combine GPU VRAM/GTT Row | Open |
| 090 | [090-remote-load-panel-uses-local-metrics.md](090-remote-load-panel-uses-local-metrics.md) | Remote Usage Load Panel Uses Local Metrics | Closed — resolved in 6522a05 |
| 091 | [091-language-agnostic-release-spec-and-thin-make-release.md](091-language-agnostic-release-spec-and-thin-make-release.md) | Language-agnostic release spec — replace per-project release Make target sprawl | Closed — resolved |
| 092 | [092-latest-release-links-and-readme-install-section-consolidation.md](092-latest-release-links-and-readme-install-section-consolidation.md) | Latest release links & README/website install section consolidation | Open — proposed assessment |
| 093 | [093-usage-tui-layout-planner.md](093-usage-tui-layout-planner.md) | Usage TUI layout planner and sizing policy | Closed |
| 094 | [094-usage-watch-controls-overlay-and-presets.md](094-usage-watch-controls-overlay-and-presets.md) | Usage Watch Controls Overlay and Presets | Closed |
| 095 | [095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display.md](095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display.md) | Restore CWD After Shell Tool Use; Surface CWD in Status Line | Open — partially resolved (statusLine shipped) |
| 096 | [096-release-forge-token-401-warning-lacks-fix-hint.md](096-release-forge-token-401-warning-lacks-fix-hint.md) | `harnez release`'s `has_releases` 401 warning doesn't name the fix | Open |
| 097 | [097-release-skip-when-no-diff-since-prev-tag.md](097-release-skip-when-no-diff-since-prev-tag.md) | `harnez release` should skip when there's no diff since the previous tag | Closed — resolved |
| 098 | [archive/098-deb-rpm-packaging.md](archive/098-deb-rpm-packaging.md) | Add DEB and RPM packaging to `harnez release` | Closed — resolved in `4f3817e` |
| 099 | [099-appimage-packaging.md](099-appimage-packaging.md) | Add AppImage packaging to `harnez release` | Closed — evaluated, not pursued |
| 100 | [archive/100-container-install-verify-canary.md](archive/100-container-install-verify-canary.md) | Container canary: install `latest` Codeberg release and verify `--version` | Closed — resolved in `7c76ec5` |
| 101 | [archive/101-usage-keep-stale-agents-visible-until-7d.md](archive/101-usage-keep-stale-agents-visible-until-7d.md) | `harnez usage` hides/blanks agents too soon when not recently active (e.g. AGY) | Closed — resolved in `74c6052`, `7c50f12` |
| 102 | [102-usage-summary-compact-should-be-allowed.md](102-usage-summary-compact-should-be-allowed.md) | `harnez usage --summary --compact` should be allowed | Closed — resolved in 1d98fb6 |
| 103 | [103-agy-missing-from-all-usage-aggregate.md](103-agy-missing-from-all-usage-aggregate.md) | Antigravity missing from All Usage aggregate despite recent historical data | Closed — resolved |
| 104 | [104-agy-quota-collector-requires-live-process-poll-coincidence.md](104-agy-quota-collector-requires-live-process-poll-coincidence.md) | AGY quota collector only records data when a live process poll coincides with a running AGY instance | Closed — resolved in 783cfcc |
| 105 | [105-surface-per-collector-fetch-status-in-usage-ui.md](105-surface-per-collector-fetch-status-in-usage-ui.md) | Surface per-collector fetch status in usage UI, including compact view | Open |
| 106 | [106-verify-offline-derivability-of-quota-state.md](106-verify-offline-derivability-of-quota-state.md) | Verify offline-derivability of quota/limit/reset-time state from current storage model | Closed — audit complete |
| 107 | [107-indicate-data-staleness-via-dimming-marker-in-usage-ui.md](107-indicate-data-staleness-via-dimming-marker-in-usage-ui.md) | Indicate data staleness via dimming/marker in usage UI (compact + full views) | Open |
| 108 | [108-subagent-dispatch-sequential-default-and-issue-number-race-guard.md](108-subagent-dispatch-sequential-default-and-issue-number-race-guard.md) | Subagent dispatch needs a hard sequential-by-default rule + issue-number allocation race guard | Open |
| 109 | [109-user-local-config-xdg-config-harnez-local-yaml.md](109-user-local-config-xdg-config-harnez-local-yaml.md) | User-local config: `~/.config/harnez/local.yaml` for machine-static overrides (default host, future caches) | Closed |
| 110 | [110-remote-load-batch-vs-streaming-collection-modes.md](110-remote-load-batch-vs-streaming-collection-modes.md) | Remote Load: batch-default with an opt-in streaming channel, graceful fallback | Closed — resolved in 1df4e0a |
| 111 | [111-per-agent-collector-pipelines-independent-cadence-and-timeout.md](111-per-agent-collector-pipelines-independent-cadence-and-timeout.md) | Per-agent collector pipelines: independent cadence, stricter timeout, and cancellation | Open |
| 112 | [112-agy-usage-poll-may-trigger-google-reauth-bot-detection.md](112-agy-usage-poll-may-trigger-google-reauth-bot-detection.md) | Investigate whether `agy -p "/usage"` polling triggers Google reauth / bot-detection dialogs | Closed — inconclusive on causation, mitigation shipped |
| 113 | [113-record-collector-roundtrip-times-usage-meta.md](113-record-collector-roundtrip-times-usage-meta.md) | Record per-collector roundtrip times; expose via `harnez usage --meta` | Open |
| 114 | [114-remote-load-stream-drops-to-batch-stdin-eof-false-trigger.md](114-remote-load-stream-drops-to-batch-stdin-eof-false-trigger.md) | Remote Load stream drops to batch shortly after connecting: stdin-EOF false-triggers shutdown | Open |
| 115 | [115-canary-duckdb-go-embedding-for-tool-telemetry.md](115-canary-duckdb-go-embedding-for-tool-telemetry.md) | Canary: evaluate DuckDB Go embedding for tool-call telemetry storage | Closed — no-go, alternative named |
| 116 | [116-tool-telemetry-schema-and-storage-layer.md](116-tool-telemetry-schema-and-storage-layer.md) | Tool telemetry schema & storage layer (`tool_calls` table) | Closed — resolved in 6cd33b0 |
| 117 | [117-harnez-rate-command.md](117-harnez-rate-command.md) | `harnez rate`: positional internal-tool rating command | Closed — resolved in 03753f4 |
| 118 | [118-harnez-exec-shell-interceptor.md](118-harnez-exec-shell-interceptor.md) | `harnez exec`: shell execution interceptor with telemetry capture | Closed — resolved in 27dca59 |
| 119 | [119-harnez-hook-agent-hook-management.md](119-harnez-hook-agent-hook-management.md) | Fold telemetry-hook install into `apply`; runtime endpoint via `harnez exec hook` | Closed — resolved in 51ebe41 |
| 120 | [120-harnez-stats-analytical-reporting.md](120-harnez-stats-analytical-reporting.md) | `harnez stats`: analytical reporting over tool_calls | Closed — resolved in 41dd27f |
| 121 | [121-multi-repo-session-and-ticket-id-resolution.md](121-multi-repo-session-and-ticket-id-resolution.md) | Multi-repo session & ticket ID resolution strategy | Closed — resolved |
| 122 | [122-agent-instruction-tool-feedback-protocol.md](122-agent-instruction-tool-feedback-protocol.md) | Agent instruction template: Tool Feedback Protocol injection | Closed — resolved in f158a98 |
| 123 | [123-session-wrapping-supervisor-for-agents-without-hook-rewrite.md](123-session-wrapping-supervisor-for-agents-without-hook-rewrite.md) | `harnez run <agent>`: session-wrapping supervisor for agents without hook-rewrite support | Open — not scheduled |
| 124 | [124-posttooluse-internal-tool-call-auto-capture.md](124-posttooluse-internal-tool-call-auto-capture.md) | `PostToolUse` hook: auto-capture internal-tool call counts without a score | Open |
| 125 | [125-long-running-agent-runs-should-use-async-feedback.md](125-long-running-agent-runs-should-use-async-feedback.md) | Long-running agent runs should use async feedback instead of chat polling | Open |
| 126 | [126-document-closed-resolved-in-commit-self-reference-convention.md](126-document-closed-resolved-in-commit-self-reference-convention.md) | Document the `Closed — resolved in <commit>` self-reference convention | Open |
| 127 | [127-move-telemetry-sql-statements-to-spec.md](127-move-telemetry-sql-statements-to-spec.md) | Move `internal/telemetry` SQL statements into `spec/` | Open |
| 128 | [128-per-agent-full-system-prompt-self-audit-for-repetition.md](128-per-agent-full-system-prompt-self-audit-for-repetition.md) | Per-agent full system-prompt self-audit for repetition and conciseness, starting with Claude Code | Open |
| 129 | [129-parallelize-usage-collectall.md](129-parallelize-usage-collectall.md) | Parallelize `collectAll`'s per-agent usage collectors (goroutines + WaitGroup) | Closed — resolved in 796f56c |
| 130 | [archive/130-instruction-distribution-audit-followups.md](archive/130-instruction-distribution-audit-followups.md) | Instruction-distribution audit follow-ups (Tool Feedback Protocol misscoping, doc-index accuracy, delivery duplication, Skill infra) | Closed — resolved in 96e7220 |
| 131 | [131-watch-debug-freshness-countdown-overlay.md](131-watch-debug-freshness-countdown-overlay.md) | `!`-toggled per-agent freshness countdown overlay in `harnez usage --watch` | Closed — resolved in `d8a3abe`, `7514ecf` |
| 132 | [132-watch-spec-driven-superscript-hotkeys.md](132-watch-spec-driven-superscript-hotkeys.md) | Spec-driven btop-style superscript hotkeys and single hidden-count hint for `harnez usage --watch` | Closed — resolved in `b23ae32`, `aee953a` |
| 133 | [133-progress-bar-eighth-block-subcharacter-precision.md](133-progress-bar-eighth-block-subcharacter-precision.md) | Sub-character precision for narrow progress bars using eighth-block glyphs | Closed — resolved in `5fd2efb`, `c7c5fdf`, `0666758` |
| 134 | [134-conversemode-to-skill-conversion.md](134-conversemode-to-skill-conversion.md) | Convert `ConciseMode.md` from AGENTS.md-mutation to a Claude-Code Skill (split from 130 item 8) | Open |
| 135 | [135-claude-code-tool-feedback-protocol-dual-delivery.md](135-claude-code-tool-feedback-protocol-dual-delivery.md) | Claude Code gets Tool Feedback Protocol content twice (global section + Skill) | Open |
| 136 | [136-bar-ansi-background-bracket-leak-and-color-spec.md](136-bar-ansi-background-bracket-leak-and-color-spec.md) | Bar ANSI background leaks onto brackets; consolidate colors into spec/ | Open |
| 137 | [137-rograph-library-boundary-and-full-color-spec-audit.md](137-rograph-library-boundary-and-full-color-spec-audit.md) | Harden `rograph` as a standalone library; move every ANSI color/style code into spec/ | Open |
