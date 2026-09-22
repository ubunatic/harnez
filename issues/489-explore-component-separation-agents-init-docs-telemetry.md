# 489 — Explore component separation: agents, init/docs, telemetry

**Status**: Closed — harnez component separation analysis in docs/HarnezComponents.md
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [Follow-up: 490 for component system design]

## /goal

Assess feasibility and benefits of decoupling harnez into separate, composable tools:
- `harnez` core → docs/skills management only
- `harnez-agents` → agent orchestration and Codex integration
- `harnez-init` → workspace/project bootstrapping
- `harnez-telemetry` → telemetry collection and hook setup

Outcome: Architecture report draft in `docs/HarnezComponents.md` identifying:
- Current dependencies between subsystems
- Which use cases require which tool combinations
- Separation boundaries and data flow
- Migration/compatibility implications

## 1. Problem & Motivation

harnez currently bundles docs management, agent orchestration, CLI bootstrapping (`init`), and telemetry into a single binary. This monolithic design creates friction for users who want:

- Selective docs/skills profiles without hooks/agent integration
- Hook setup and telemetry only (managing docs elsewhere)
- Agent functionality in isolation (with direct token cost tracking)
- Different update/deployment cadences per subsystem

Current architecture makes it hard to mix harnez docs with non-harnez agents or use harnez agents with different doc sources.

## 2. Technical Specification / Findings

Explore the codebase to:

1. **Map current component dependencies**: trace how `agents/`, `init/`, and `telemetry/` interact with core docs/skills functionality
2. **Identify extraction seams**: which interfaces/data structures would need to be public if separated
3. **Assess independent deployability**: could each tool be versioned and distributed separately?
4. **Enumerate use case coverage**: map existing user workflows to subsystem combinations
5. **Document findings** in `docs/HarnezComponents.md` (sections: architecture, coupling analysis, proposed boundaries, migration notes)

## 3. Implementation & Verification Plan

- [x] Read existing architecture docs (`docs/`, `docs/lang/`, internal subsystem boundaries)
- [x] Grep/explore codebase for cross-subsystem imports and data flow
- [x] Draft component dependency matrix (which subsystems call which)
- [x] Identify integration points (CLI flags, config files, environment variables)
- [x] Write findings report in `docs/HarnezComponents.md`
- [x] (No code changes in this ticket; findings-only)
- [x] Commit: `docs(issues): close 489, harnez component separation analysis`

## Findings

Report: `docs/HarnezComponents.md`. Summary:

- `internal/` coupling is low; the only cross-subsystem package edges are `claude → issues.Lint`,
  `claude → usage.LoadLocalConfig`, `subagent → readcard`, and shared `harnez.DefaultFS` spec files.
- Real coupling sits in `cmd/harnez` (single `main` package) and runtime contracts: hook command
  strings written by `apply`, skill text naming other components' CLIs, the shared
  `~/.harnez/tool_catalog.sqlite` store, and the single `config.yaml`.
- The proposed four-way split omits two large subsystems: the usage/quota monitor and the issue tracker.
- Extraction order by cost: agents, issue tracker, usage monitor, telemetry. Init stays with core
  (it shares apply's config and doc machinery).
- Telemetry is effectively the "hooks" tool: distill, quota1, and read discipline must move with it
  because each hook is a single composed process.
- Lowest-risk path: git-style `harnez <cmd>` dispatcher plus per-component binaries from the same module.
