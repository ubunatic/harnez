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
| 023 | [023-usage-command-token-quota-tracking.md](023-usage-command-token-quota-tracking.md) | `harnez usage`: unified token, session & quota status command | Implemented — `--watch`, `--summary`, robustness fixes committed 2026-08-18 |
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
| 043 | [043-never-blindly-revert-commit-stale-work-first.md](043-never-blindly-revert-commit-stale-work-first.md) | Never blindly `git revert`/`checkout --`/`stash drop` unsuccessful work; commit stale/failed code first | Open |
