# 489 — Explore component separation: agents, init/docs, telemetry

**Status**: Open
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

- [ ] Read existing architecture docs (`docs/`, `docs/lang/`, internal subsystem boundaries)
- [ ] Grep/explore codebase for cross-subsystem imports and data flow
- [ ] Draft component dependency matrix (which subsystems call which)
- [ ] Identify integration points (CLI flags, config files, environment variables)
- [ ] Write findings report in `docs/HarnezComponents.md`
- [ ] (No code changes in this ticket; findings-only)
- [ ] Commit: `docs(issues): close 489, harnez component separation analysis`
