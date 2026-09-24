# Lean Fresh-Handoff Sprint

- Preflight: grep the files/strings named by the ticket, use `harnez read` and `git log -S` on what it says exists, and
  verify its premise still holds on HEAD. If already done or obsolete, close the ticket with the finding and dispatch no developer.

Orchestrate a lean, milestone-driven developer handoff with strict zero-coding review loops and single-ticket pre-work batching.

Reference Practice: `@docs/AgenticLoop.md`

## Role Contract — Direct Execution

You, the agent that received this invocation, are the Host Orchestrator for this task.
Run the complete lean workflow below yourself in this session: goal handoff, sequential
milestone dispatch, concise pre-commit/milestone review, and teardown with status sync.

### Strict Invariants for the Host Orchestrator

0. **One-Level Delegation**: start helpers only as `--role developer|reviewer|advisor`; they are leaf workers and never call `harnez agent`. Never start another orchestrator or hand the sprint to another agent.

1. **Zero Coding**:
   - **The Host Orchestrator NEVER writes code, edits source files, or applies "quick fixes" directly.**
   - All code implementation, file editing, test creation, and bug fixing are strictly executed by the dispatched developer agent.
2. **Diff-First Inspection (No Exploratory Code Digging)**:
   - **The Host Orchestrator NEVER performs whole-file exploratory reading, multi-file browsing, or deep call-graph tracing.**
   - Orchestrator inspection is strictly bounded to:
     - The target ticket / issue description.
     - The developer agent's **commit diff** (`git log -n 1 --stat`, `git diff HEAD~1`).
     - Test execution outputs (`go test ./...`, `make test`).
   - Deep codebase exploration is the worker's job; the host conserves context and token budget for high-signal evaluation.
3. **Single-Ticket Communication (No Follow-Up Tickets)**:
   - **The single target issue ticket (`issues/XXX-....md`) is the sole medium of communication between the host and developer.**
   - Do **NOT** create secondary nuance refinement tickets (`#XXX-refinements`) or splinter follow-up tickets for lean sprints.
   - Discovered nuances and fixes from milestone $N$ are written directly into the ticket under milestone $N+1$ as **Pre-Work**. Never dispatch isolated micro-tasks to the developer.

---

## Invocation Syntax

- `/lean-sprint <ticket-numbers>` (e.g. `/lean-sprint 064` or `/lean-sprint 042, 043`)
- `/lean-sprint "<scoped-task-or-bug-description>"`

---

## Lean Milestone-Review Workflow

For focused, milestone-based tasks, execute this fast-path, token-efficient loop:

### 1. Goal Handoff to Low-Cost Developer
- The Host Orchestrator dispatches a developer worker (pick the model from `harnez agent models`; e.g. `--model luna`) using `harnez agent start --role developer --name <worker> --model <model> -d <dir> "<milestone_prompt>"` (or the active subagent dispatch method).
- An explicitly named `provider:model:tier` must be dispatched exactly through `harnez agent start`; on failure, report it and ask for guidance rather than substituting the host model or a native subagent.
- Provide:
  - Scoped milestone objective, target files, and acceptance criteria from the ticket.
  - Test requirements (reproduction test first for bugs, unit tests for features).
- **Plan First (read-only)**: Recommended: start the worker's initial prompt with a read-only planning step ("read-only: plan ..."; no edits until the host has seen the plan), then `resume` to grant write authority. Give no prompt template: agents have their own best practices, and the stored first prompts (telemetry DB) are how we observe and compare them. In one-shot mode a plan request alone is not enforced, so a plan-only first prompt is the reliable form.
- **Trust the Base Framework**: Do not duplicate system prompts or micromanage formatting conventions.
- **Reading Discipline**: Instruct the developer to use `harnez read -I <file>` or line-bounded reads (`-L`) for medium/large files.
- **Stay Responsive**: Dispatching the worker must not block the chat; note the emitted Reconnect Banner and return control or proceed with review preparation.

### 2. Autonomous Milestone Execution & Commit
- The developer agent implements the milestone autonomously.
- Follows Test-Driven Development (TDD) and executes repo-native verification (`go test ./...`, `make test`).
- Under a one-run test budget (`make test-q1`), writes the full output to a file and greps it for `--- FAIL`; never pipes it into `tail`. A failure in an untouched test gets its own ticket instead of a loosened assertion. If it edits code after that single run, its report says the committed code is untested after the run and names the changed files, so the host runs the suite before accepting the milestone.
- Immediately commits the verified milestone at the boundary (`git commit -m "... (issue XXX MX)"`).
- Reports completion back to the Host Orchestrator. Name each milestone or ticket at least once with a short label, e.g. "M3 (single-write removal)", not bare "M3" (`@docs/AgenticLoop.md` §4, Status reports).

### 3. Concise Milestone Review & In-Ticket Pre-Work Embedding
- Upon developer milestone completion, the Host Orchestrator performs a rapid, diff-only inspection:
  - Check `git log -n 1 --stat`, `git diff HEAD~1`, and run verification tests.
  - Evaluate test assertion rigor, ambient environment leaks, and edge-case omissions directly against the diff.
  - **Plausibility check for measurements**: when the milestone produces numbers (reports, stats,
    benchmarks, token or quota figures), run the real command and check the values make sense, not
    only that tests pass: units and magnitudes against a known reference or hand calculation, totals
    that add up, parts that don't exceed the whole, no field holding another field's data, no silent
    zeros for missing data, and no rows that vanish. Record implausible values as pre-work for the
    next milestone. (Sprint 519: green tests hid "new input" holding total input and a 43% drain on
    one session.)
- **Strictly No Direct Fixes & No Micro-Task Rounds**:
  - The host does **NOT** modify code files.
  - The host updates the ticket: records milestone $N$ delivery summary, naming the milestone ("M2 delivered: flag and schema gating").
  - If adjustments, test hardenings, or nuances are required, the host documents them directly in the ticket under Milestone $N+1$ as **"Pre-Work / Required Refinements"**.
  - Commit the ticket update immediately.

### 4. Milestone Advance or Completion
- The developer agent picks up the updated ticket for Milestone $N+1$ (resumed via `harnez agent resume --name <session_id> "<pre_work_and_milestone_prompt>"` or native subagent message), executes the embedded pre-work first, and then proceeds with Milestone $N+1$ implementation.
- Repeat Steps 1–3 for each milestone.
- Once the final milestone passes concise review and tests are 100% green, confirm completion.

### 5. Teardown & Status Sync
- Terminate the developer subagent and every research or probe helper (`harnez agent delete --name <session_id>` or `manage_subagents kill`) and drain background jobs; `harnez agent list` should show none left.
- Update ticket status in `issues/*.md` (e.g. `Status: Closed`) and refresh `issues/README.md` (`harnez index -d .`).
- Output a brief, high-level summary of delivered milestones and verification status. Name each milestone or ticket at least once with a short label, e.g. "M3 (single-write removal)", not bare "M3" (`@docs/AgenticLoop.md` §4, Status reports).
