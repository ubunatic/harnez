# 364 — Research Quota-1 guardrails for agent loops via harnez init

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [issues/023-usage-command-token-quota-tracking.md](archive/023-usage-command-token-quota-tracking.md), [issues/106-verify-offline-derivability-of-quota-state.md](106-verify-offline-derivability-of-quota-state.md), [issues/293-make-roadmap-synthesis-recoverable-across-quota-interruptions.md](293-make-roadmap-synthesis-recoverable-across-quota-interruptions.md)

---

## 1. Problem & Motivation

Autonomous and semi-autonomous LLM agent loops (such as iterative test-fix runners, multi-step refactoring workflows, subagent orchestrators, and automated sprint execution) frequently run until they hit a hard quota or rate-limit ceiling. 

When an agent hits a hard quota boundary mid-turn:
1. **Uncommitted / Incomplete State**: Changes may be left uncommitted or unindexed without a clean commit checkpoint.
2. **Zombie Subagents & Background Tasks**: Child processes or scheduled timers remain active or orphaned because the parent agent could not execute its Phase 4 hygiene/cleanup.
3. **No Graceful Handoff**: The agent fails mid-operation with an unhandled API error rather than creating a resumption marker for the next session.

The **Quota-1** design pattern addresses this by maintaining a safety floor (reserving at least 1 turn / unit of quota before exhaustion). When the floor is reached, the agentic loop is proactively halted, allowing the agent to run final cleanups, commit in-progress artifacts, document remaining work, and shut down cleanly before hard exhaustion.

This research ticket investigates how `harnez init` can scaffold and inject Quota-1 guardrails directly into target repositories.

---

## 2. Quota-1 Concept & Failure Modes

### 2.1 The Quota-1 Pattern
- Instead of running a loop until an HTTP 429 / quota error is thrown, the loop driver or agent harness monitors remaining rate/quota windows (e.g. via `harnez usage` or quota collectors).
- When remaining capacity hits the threshold (e.g., remaining quota $\le 1$ or below a safety buffer percentage), the loop transitions from **Productive Dispatch** to **Graceful Drain**:
  - Halts new task/subagent dispatches.
  - Completes and commits current discrete steps.
  - Serializes state / resumption checkpoints.
  - Cleans up background tasks and timers.

### 2.2 Key Failure Modes to Guard Against
- **Mid-Edit Interruption**: Quota cutoffs during file writes leaving broken syntax or partial edits.
- **Deadlocked Subagents**: Inability of a parent to receive child responses or send follow-ups.
- **Lost Context on Re-entry**: Future agents resuming without knowing where the previous agent stopped.

---

## 3. Potential Integration Points for `harnez init`

`harnez init` configures project-level scaffolding (`AGENTS.md`, `Makefile`, copyable practice docs). Potential integration mechanisms include:

1. **`AGENTS.md` / `AgenticLoop.md` Invariant Addition**:
   - Add a canonical invariant (e.g. *Invariant 8: Quota-1 Loop Guardrail*) instructing agents running iterative loops to check quota headroom before starting multi-turn subtasks, and to trigger graceful drain when near quota boundaries.
2. **Makefile Scaffolding**:
   - Add pre-flight targets or wrapper recipes (e.g. `make loop-check`, `make agent-preflight`) that query `harnez usage --json` / exit non-zero if quota is insufficient.
3. **Task & Sprint Tooling Integration**:
   - Update `/sprint`, `harnez issues`, and autonomous script templates to respect Quota-1 boundaries and emit structured resumption metadata on pause.
4. **Project Pre-Execution Hooks / Shims**:
   - Scaffolding project-local scripts or hooks that verify quota availability prior to triggering heavy background agent jobs.

---

## 4. Research Questions & Exploration Scope

1. **Telemetry & Query Latency**:
   - Can `harnez usage` (or the underlying usage storage / cache) provide sub-100ms quota checks suitable for pre-loop evaluation without adding significant overhead?
2. **Cross-Harness Posture**:
   - How do Claude Code, Codex, and AGY report quota exhaustion, and can a unified Quota-1 threshold function across all three?
3. **Scaffolding UX in `harnez init`**:
   - Should Quota-1 guardrails be default-on in `harnez init` templates (Makefile + AGENTS.md), or opt-in via flags (e.g. `harnez init --guardrails`)?
4. **Clean Resumption Protocol**:
   - Define the exact schema for resumption tickets/markers when a Quota-1 drain occurs mid-sprint.

---

## 5. Acceptance Criteria

- [ ] Survey existing quota-monitoring mechanisms in `internal/usage/` and assess suitability for fast loop pre-flight checks.
- [ ] Define the canonical Quota-1 invariant and draft updates for `docs/practices/AgenticLoop.md`.
- [ ] Design the `harnez init` scaffolding changes (Makefile targets and `AGENTS.md` rules).
- [ ] Prototyping & Canary: Test a Quota-1 loop guardrail against a simulated quota exhaustion scenario.
- [ ] Produce an implementation ticket or PR for incorporating the guardrails into `harnez init`.
