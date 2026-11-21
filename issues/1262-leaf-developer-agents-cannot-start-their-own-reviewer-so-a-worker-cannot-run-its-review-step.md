# 1262 — Leaf developer agents cannot start their own reviewer, so a worker cannot run its review step

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 486, 523, 953, 1107, 1263; study `docs/studies/2026-11-21-headless-signal-assistant-one-day-build-story.md`

---

## 1. Problem & Motivation

A developer worker is a leaf role. It cannot start a reviewer on its own commit, so the review step has to be started by the orchestrator. In a headless run that adds a round trip after every commit and makes the review depend on the orchestrator staying alive.

Evidence: operator report from 2026-11-21/22, when a headless orchestrator ran harnez workers. The refused attempt itself is **unverified**: the study (`docs/studies/2026-11-21-headless-signal-assistant-one-day-build-story.md`) records reviews started by the orchestrator, not a worker refusal. Verify by reading the developer's provider transcript for a `harnez agent start --role reviewer` call and its refusal text.

## 2. Technical Specification / Findings

- Developer is a leaf: `spawns: []` at `spec/agent.yaml:438`. Reviewer is a leaf too (`spec/agent.yaml:456`). Orchestrator has `spawns: [developer, reviewer, advisor]` (`spec/agent.yaml:411`).
- Refusal: `internal/subagent/agentspec.go:836-841`: "agent role %q is a leaf worker: it must not start, resume or manage agents; ... the host runs live harnez agent checks after your commit".
- `IsLeafRole` (`internal/subagent/agentspec.go:852-858`) treats an empty `spawns` list as leaf. `guardLeafRole` (`cmd/harnez/agent_run.go:1820-1822`) uses it for mutating verbs; wired from `cmd/harnez/agent.go:134` and `:223`.
- The guard is intentional: 486 (enforced leaf workers, closed) and 523 (lean-sprint: the host runs live checks, closed) record the decision. This ticket asks whether one edge should open, not whether the guard is a bug.
- The per-role allow-list already exists and is enforced at `agentspec.go:843-848`. Whether `spawns: [reviewer]` on developer also lifts `guardLeafRole` for other verbs is **unverified**.

## 3. Implementation & Verification Plan

Owner decision first:

- **A.** Keep the guard. Document in the orchestrator contract (AgenticLoop or the lean-sprint command) that the orchestrator starts the reviewer after each developer commit. Doc-only.
- **B.** Allow `developer` to start `reviewer` only (`spawns: [reviewer]`). Keep `reviewer` as a leaf so depth stays at two. Needs a canary first: a developer run of `harnez agent start --role reviewer` succeeds, and `--role developer` is still refused. Update the leaf tests from 486 and 1107.

Recommendation: B, limited to that one edge, because the review round trip is the bottleneck in headless runs. Verify with the canary before any code change. Quota-1 applies to the test run.
