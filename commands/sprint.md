# Orchestrate Agentic Sprint Loop

Orchestrate a structured, 5-phase agentic sprint loop across specified tickets, issues, or feature goals.

Reference Practice: `@docs/AgenticLoop.md`

---

## Invocation Syntax

- `/sprint <ticket-numbers>` (e.g. `/sprint 003, 004, 011` or `/sprint 039`)
- `/sprint "<feature-goal-or-task-description>"`

---

## 5-Phase Sprint Workflow

Follow these 5 phases sequentially:

### Phase 1: Parallel Advisory Discovery (Concurrent Read-Only)
1. Parse the target tickets or goals from the prompt.
2. Spawn concurrent read-only advisor subagents (`invoke_subagent`), one per target ticket or distinct subsystem.
3. Instruct advisors to:
   - Audit problem statements in `issues/` and related code paths using targeted `grep_search` and range-bounded reads (avoid whole-file reads on `AGENTS.md` or active prompt rules).
   - Check whether work is already completed or if prior assumptions changed.
   - Identify target files, exact line ranges, and test requirements.
   - Formulate a clean, step-by-step implementation plan.
4. Collect and synthesize advisor findings into a unified, conflict-free sprint plan.
5. Present the synthesized plan and task sequence to the user.
6. Keep the host orchestrator responsive throughout delegation. Do not block the main chat on subagent waits unless the user explicitly asked to wait or the next integration step is blocked on a child result.

### Phase 2: Sequential Development & Test Verification (Single-Threaded)
1. Process tasks one by one in sequence (avoid concurrent edits to the same codebase/worktree).
2. Follow Test-Driven Development (TDD):
   - Add or update unit tests alongside or before modifying implementation code.
   - Run tests (`go test -count=1 ./...`, `make test`) to verify each milestone before moving to the next.
   - Maintain codebase stability, ensuring clean compilation at every step.
   - For defect-shaped tickets: establish a concrete reproduction baseline *before* coding the fix (see `@docs/AgenticLoop.md` Phase 2, "Repro-before-fix").

### Phase 3: Pre-Commit Review Gate (Independent Reviewer)
1. Spawn an independent reviewer subagent (or conduct a dedicated review pass).
2. The reviewer audits the full git diff (`git diff`, `git status`) and verifies:
   - **Test Assertion Rigor**: Are assertions meaningful, robust, and verifying real behaviors?
   - **Documentation & Tracker Sync**: Are `issues/*.md` statuses, `issues/README.md`, `docs/README.md`, and `AGENTS.md` updated?
   - **Invariants & CLI Design**: Are command boundaries (`apply` vs `init`) and architectural rules respected?
   - **Media & Demo Verification**: If reels, WebM files, or UI screenshots were produced, has explicit user confirmation been obtained before publishing?
   - **Code Cleanliness**: Is the code token-efficient, idiomatic, and minimal?
3. Address any review findings before proceeding to commit.

### Phase 4: Process & Subagent Hygiene (Teardown & Drain)
1. Inspect running background tasks (`manage_task list`).
2. Explicitly terminate completed, idle, or zombie background tasks, schedule timers, and watch subprocesses.
3. Clean up subagents (`manage_subagents kill_all` or targeted `kill`).
4. Ensure no background processes are left running unmonitored.

### Phase 5: Agentic Flow Quality Retrospective (Feedback & Tracker Sync)
1. Record session flow learnings, tooling friction, or agent harness feedback:
   - Add retrospective notes to `docs/feedback/<YYYY-MM-DD>-<topic>.md` or run `/story` if a major case study was produced.
2. Synchronize the issue tracker:
   - Update issue status in `issues/*.md` (e.g. `Status: Closed`).
   - Update `issues/README.md` table.
   - Run `harnez status` to ensure all issue statuses are clean and validated.
   - When closing a ticket, check the whole file for multiple status-bearing fields and update all (see `@docs/AgenticLoop.md` Phase 5, "Single Status field").
3. Present a clear, concise summary of completed tickets and accomplishments to the user.
