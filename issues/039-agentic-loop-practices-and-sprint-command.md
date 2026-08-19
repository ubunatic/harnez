# 039 — `docs/practices/AgenticLoop.md` and `/sprint` Command Scaffolding

**Status**: Closed  
**Category**: Feature / Agentic Orchestration  
**Related**: [Study: 2026-08-19 Subagent Lifecycle & Friction](../docs/studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md), [Feedback: 2026-08-19](../docs/feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md), [AGENTS.md](../../AGENTS.md)

---

## 1. Problem & Motivation

The agentic development cycle developed during recent maintenance sprints operates across five distinct phases:
1. **Parallel Advisory Discovery**: Concurrent read-only subagents audit problem scope, analyze code paths, and refine tickets without code collisions.
2. **Sequential Development & Test-Driven Verification**: Single-threaded implementation to preserve test suites, maintain idempotency, and avoid git merge conflicts.
3. **Pre-Commit Review Gate**: Independent reviewer subagent auditing test assertion rigor, docs/ticket sync, and code clarity.
4. **Process & Subagent Hygiene**: Explicit inspection and teardown of background entities, timers, and zombie subprocesses.
5. **Agentic Flow Quality Retrospective**: Documenting the quality, friction, velocity, and learnings of the agentic flow directly in the repository (`docs/feedback/` or `docs/studies/`).

Currently, these phases exist only across conversational transcripts and scattered rules. There is no dedicated practice document defining this loop for agents, nor is there a registered slash command/skill (`/sprint`) to trigger and orchestrate it across AI coding tools (Claude Code, Gemini, Codex, Prime Agent).

---

## 2. Detailed Technical Specification

### 2.1 Practice Document: `docs/practices/AgenticLoop.md`
- **Location**: `docs/practices/AgenticLoop.md` (copyable practice doc).
- **Structure & Content**:
  1. **Core Philosophy & Invariants**:
     - *Parallel Read, Sequential Write*: Never allow multiple concurrent agents to edit overlapping files or dirty git worktrees.
     - *Canary & Test Verification*: Never declare a task complete without executing full test suites (`go test ./...`, `scripts/smoke-test.sh`).
     - *Zero Zombie Guarantee*: Every process, timer, or subagent spawned must be explicitly accounted for and cleanly terminated.
  2. **5-Phase Loop Execution**:
     - **Phase 1: Parallel Advisory Discovery**:
       - Host spawns concurrent read-only advisors per ticket/feature to evaluate feasibility, locate target lines, verify whether work is already done, and outline implementation steps.
       - Advisors communicate findings via message or structured summary back to the host.
     - **Phase 2: Sequential Development & TDD**:
       - Host or dedicated dev subagent executes tasks one at a time.
       - Add/update tests first or alongside fix.
       - Verify test suites at every milestone before proceeding to the next ticket.
     - **Phase 3: Pre-Commit Review Gate**:
       - An independent reviewer subagent (or separate review pass) audits the git diff.
       - Checks: assertion rigor (not just passing tests), docs/ticket synchronization, backward compatibility, and token-efficient code clarity.
     - **Phase 4: Process & Subagent Hygiene**:
       - Audit background task table (`manage_task list`, background jobs).
       - Explicitly kill completed, idle, or lingering tasks and schedules.
       - Ensure working tree is clean except for intentional modifications.
     - **Phase 5: Agentic Flow Quality Retrospective**:
       - Record session retrospectives in `docs/feedback/` (agentic patterns/friction) or `docs/studies/` (in-depth engineering case studies via `/story`).
       - Update issue tracker status (`issues/README.md`) and run `harnez status` to ensure consistency.
  3. **Role Taxonomy**:
     - **Host Orchestrator**: Maintains high-level plan, sequences dev work, coordinates subagent lifecycles, and interfaces with the user.
     - **Ephemeral Advisor**: Read-only specialist for audits, exploratory grep, and ticket refinement.
     - **Dev Worker**: Focused single-task implementer writing clean code and tests.
     - **Independent Reviewer**: Critical reviewer evaluating diffs against requirements and quality gates.

### 2.2 Command & Multi-Agent Skill: `commands/sprint.md`
- **File**: `commands/sprint.md`
- **Prompt Guidance**:
  - Accepts targeted issue numbers or task descriptions (e.g. `/sprint 003, 004, 011` or `/sprint "Implement feature X"`).
  - Guides the host agent through all 5 phases systematically:
    1. Spawn parallel read-only advisory subagents to audit tickets and confirm requirements.
    2. Review advisor reports, synthesize a unified execution plan, and present it to the user.
    3. Execute implementation sequentially with test verification (`make test`, `make smoke`).
    4. Spawn an independent reviewer subagent to audit git diff before committing.
    5. Clean up background tasks (`manage_task`) and subagents (`manage_subagents`).
    6. Record learnings in `docs/feedback/` or `docs/studies/`, and synchronize `issues/README.md`.

### 2.3 Registration in `config.yaml`
- **Commands**:
  ```yaml
  - name: sprint
    description: "Orchestrate a 5-phase agentic sprint loop (Advisors -> Dev -> Review -> Hygiene -> Retro)"
    file: commands/sprint.md
  ```
- **Skills** (shared across Gemini, Codex, Prime Agent):
  ```yaml
  - name: sprint
    description: "Orchestrate a 5-phase agentic sprint loop (Advisors -> Dev -> Review -> Hygiene -> Retro)"
    file: commands/sprint.md
  ```
- **Languages / Docs**:
  Register `agentic-loop` in `agents_md.languages` (or copyable practice doc entry) with source `docs/practices/AgenticLoop.md` and target `~/.claude/docs/AgenticLoop.md`.

### 2.4 Wiring into Root Documents & Conventions
- **`AGENTS.md`**:
  - Reference `@docs/AgenticLoop.md` in `Language Conventions` / practices section.
  - Summarize the 5-phase workflow in the `Development & Review Workflow` section.
- **`docs/README.md`**:
  - Add `docs/practices/` category table indexing `practices/AgenticLoop.md`.

---

## 3. Implementation & Verification Plan

1. **Scaffold Practice Doc & Command**:
   - Create `docs/practices/AgenticLoop.md`.
   - Create `commands/sprint.md`.
2. **Update Configuration & Pipeline**:
   - Add `sprint` to `commands` and `skills` in `config.yaml`.
   - Register `agentic-loop` under `agents_md.languages` in `config.yaml`.
   - Wire reference into root `AGENTS.md` and index in `docs/README.md`.
3. **Verification**:
   - Run `go test ./...` to ensure all parser and generator unit tests pass.
   - Run `make install` to compile the updated binary to `~/go/bin`.
   - Run `go run ./cmd/harnez status` to verify issue status linter passes with zero warnings.
   - Run `bash scripts/smoke-test.sh` to confirm idempotent application and drift detection across all platforms.

