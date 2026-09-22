# 490 — Design harnez component system: selective profile composition

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [489: Component separation exploration; findings in docs/HarnezComponents.md]

## /goal

Design and prototype a composable component system for harnez that allows users to:
- Use selective docs/skills profiles without hooks or agent integration (when using non-harnez agents)
- Setup telemetry and hooks only (managing docs/skills externally)
- Use harnez agents in isolation with direct token cost tracking per session
- Customize which subsystems activate based on project/user profile

Outcome: Updated architecture in `docs/HarnezComponents.md` with:
- Component composition model and API
- Profile selection mechanism (`apply --profile=<name>`)
- Data flow and initialization order
- Migration path from monolithic to modular design

## 1. Problem & Motivation

Users have different deployment models:

1. **Docs-only users**: Want harnez-managed docs and skills without agent hooks or telemetry
2. **Telemetry-only users**: Need hook setup and metrics collection; manage docs elsewhere
3. **Agent-focused users**: Use harnez for agent orchestration with direct cost tracking; docs from other sources
4. **Hybrid setups**: Mix different tool versions or sources across subsystems

The current monolithic binary forces all-or-nothing adoption. A component system would support these use cases independently while maintaining compatibility for integrated users.

## 2. Technical Specification / Findings

Based on 489 findings in `docs/HarnezComponents.md`:

1. **Define component boundaries**: Isolate docs/skills, agents, telemetry, and init into independently deployable units
2. **Design profile system**: 
   - `docs-only`: apply selective docs/skills (no hooks, no agents)
   - `telemetry-only`: hooks and metrics (no docs/agent runtime)
   - `agents-only`: agent orchestration (no doc management)
   - `full` (default): integrated experience
3. **Specify composition API**: How components discover and initialize each other
4. **Plan initialization order**: Ensure safe startup regardless of which components are active
5. **Document migration path**: How existing users transition to profiles

## 3. Implementation & Verification Plan

- [x] Review 489 findings in `docs/HarnezComponents.md`
- [x] Design component lifecycle and initialization contract (§8.4, §8.7)
- [x] Define profile YAML structure and selection logic (§8.3, §8.5; named `components`, not `profile`)
- [x] Draft API signatures for cross-component communication (§8.6: file/CLI contracts)
- [x] Prototype profile system with small integration test (MVP: `internal/components`, `scripts/canary-components`; main files unchanged)
- [x] Update `docs/HarnezComponents.md` with design details (§8, incl. alternatives §8.1)
- [x] Design review checklist: can each profile boot independently? Can they compose safely? (MVP tests apply every preset and a preset union, §8.10)
- [ ] Commit changes to docs and prototype code

## Design status

Design written in `docs/HarnezComponents.md` §8: five paths compared (status quo, component
selection in the monolith, multi-binary, separate repos, component manifests); recommended
path A now with triggers for B/C/D. MVP in `internal/components` backs the design without
touching `internal/claude` or `cmd/harnez`; integration into apply is pending review of
the design and the open questions in §8.9 (gap table in §8.10).
