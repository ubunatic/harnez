# Lean Fresh-Handoff Sprint

Orchestrate a lean, milestone-driven developer handoff with strict zero-coding review loops.

Reference Practice: `@docs/AgenticLoop.md`

## Role Contract — Direct Execution

You, the agent that received this invocation, are the Host Orchestrator for this task.
Run the complete lean workflow below yourself in this session: goal handoff, sequential
milestone dispatch, concise pre-commit/milestone review, and teardown with status sync.

### Strict Invariant: Zero Coding for the Host Orchestrator
- **The Host Orchestrator NEVER writes code, edits source files, or applies "quick fixes" directly.**
- All code implementation, file editing, test creation, and bug fixing are strictly executed by the dispatched developer agent.
- The Host Orchestrator acts purely as the high-capability reviewer, tester, and milestone coordinator, preserving context and token quota for concise evaluation.

---

## Invocation Syntax

- `/lean-sprint <ticket-numbers>` (e.g. `/lean-sprint 064` or `/lean-sprint 042, 043`)
- `/lean-sprint "<scoped-task-or-bug-description>"`

---

## Lean Milestone-Review Workflow

For focused, milestone-based tasks, execute this fast-path, token-efficient loop:

### 1. Goal Handoff to Low-Cost Developer
- The Host Orchestrator dispatches a developer worker (selecting a fast/low-cost model, e.g. Codex Luna or lightweight model).
- Provide:
  - Scoped milestone objective, target files, and acceptance criteria.
  - Test requirements (reproduction test first for bugs, unit tests for features).
- **Trust the Base Framework**: Do not duplicate system prompts or micromanage formatting conventions.
- **Reading Discipline**: Instruct the developer to use `harnez read -I <file>` or line-bounded reads (`-L`) for medium/large files.
- **Stay Responsive**: Dispatching the worker must not block the chat; return control or proceed with review preparation.

### 2. Autonomous Milestone Execution & Commit
- The developer agent implements the milestone autonomously.
- Follows Test-Driven Development (TDD) and executes repo-native verification (`go test ./...`, `make test`).
- Immediately commits the verified milestone at the boundary (`git commit -m "... (issue XXX MX)"`).
- Reports completion back to the Host Orchestrator.

### 3. Concise Milestone Review & Nuance Injection
- Upon developer milestone completion, the Host Orchestrator performs a rapid, concise inspection:
  - Check `git log -n 1 --stat`, `git diff HEAD~1`, and run verification tests.
  - Evaluate test assertion rigor, ambient environment leaks, and edge-case omissions.
- **Strictly No Direct Fixes**: If defects, gaps, or nuances are discovered:
  - Do **NOT** modify code files yourself.
  - Summarize the concrete issues, edge cases, and required adjustments concisely.
  - **Direct Nuance Forwarding**: Inject these findings directly as prioritized requirements into the **next milestone prompt** for the developer agent to resolve in their next cycle.

### 4. Milestone Advance or Completion
- Repeat Steps 1–3 for each milestone defined in the ticket.
- Once the final milestone passes concise review and tests are 100% green, confirm completion.

### 5. Teardown & Status Sync
- Terminate the developer subagent and drain background jobs.
- Update ticket status in `issues/*.md` and refresh `issues/README.md` (`harnez index -d .`).
- Output a brief, high-level summary of delivered milestones and verification status.
