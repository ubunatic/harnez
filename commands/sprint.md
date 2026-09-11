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

### Phase 1: Sequential Advisory Discovery (One Reusable Advisor)
1. Parse the target tickets or goals from the prompt.
2. Reuse the current advisor session if available; a closed or parked advisor remains eligible for native reuse. Start a new reusable advisor with a frontier model only when no compatible session exists or the existing one has an explicit health/compatibility failure, such as the Codex usage-limit dead-session behavior. Record its session ID and model. Use this same advisor for every ticket, one at a time; never dispatch the next ticket before the previous ticket's compaction completes.
3. Hand the advisor one ticket or bounded goal at a time. Instruct it to:
   - Audit problem statements in `issues/` and related code paths using targeted `grep_search` and range-bounded reads (avoid whole-file reads on `AGENTS.md` or active prompt rules).
   - Check whether work is already completed or if prior assumptions changed.
   - Identify target files, exact line ranges, and test requirements.
   - Formulate a clean, step-by-step implementation plan.
   - Recommend broader subsystem/work categories, short technical developer names (e.g. `cli`, `spec`, `docs`), and a suitable model for each. Explicitly justify any recommendation to use the top frontier model for development.
   - Work within the requested advice scope: it may edit tickets, create documents under `docs/` or `issues/`, and make tiny 3–4 line fixes only when the build stays clean. Verify the build for such fixes. Do not perform normal coding or split a larger change into tiny fixes to bypass this limit; hand normal implementation to developers.
   - Make intermediate commits when reasoning locks in substeps of the requested advice, committing only its scoped changes and observing the applicable review requirements. Persist conclusions, decisions, and handoff details in tickets/docs before compaction.
4. Collect each ticket's findings and durable references into the orchestrator's sprint plan. After each ticket, the orchestrator must explicitly call the available session-compaction operation on the advisor session and confirm completion. This includes an explicit call after the last ticket, even if no further work is queued. An instruction to the advisor to compact itself is not a substitute. Keep the same session for clean-but-cached reuse, including hours later; do not replace it with a fresh advisor per ticket.
5. Present the synthesized plan and task sequence to the user.
6. Keep the host orchestrator responsive throughout delegation. Do not block the main chat on subagent waits unless the user explicitly asked to wait or the next integration step is blocked on a child result.

### Phase 2: Sequential Development & Test Verification (Reusable Developers)
1. Create or reuse one developer agent per broader subsystem/work category from the advisor's plan, rather than one per ticket. Give each a short technical name users can refer to, and record its name, session ID, category, and model. Select a suitable lower-cost model; use the top frontier model only when the advisor explicitly recommends it.
2. Process tasks one by one in sequence through the matching named developer (avoid concurrent edits to the same codebase/worktree). Reuse that session for later work in its category, supplying bounded tasks and durable references.
3. Follow Test-Driven Development (TDD):
   - Add or update unit tests alongside or before modifying implementation code.
   - Run tests (`go test -count=1 ./...`, `make test`) to verify each milestone before moving to the next.
   - Maintain codebase stability, ensuring clean compilation at every step.
   - For defect-shaped tickets: establish a concrete reproduction baseline *before* coding the fix (see `@docs/AgenticLoop.md` Phase 2, "Repro-before-fix").
4. After each larger work item, persist its learnings in issues/docs/code and pass the Phase 3 review gate before committing implementation. Then the orchestrator must make an explicit final session-compaction call for that developer and confirm completion before handing it another item or parking it. Merely telling the developer to compact at the end does not satisfy this checkpoint.

### Phase 3: Pre-Commit Review Gate (Independent Reviewer)
1. Spawn an independent reviewer subagent (or conduct a dedicated review pass).
2. The reviewer audits the full git diff (`git diff`, `git status`) and verifies:
   - **Test Assertion Rigor**: Are assertions meaningful, robust, and verifying real behaviors?
   - **Documentation & Tracker Sync**: Are `issues/*.md` statuses, `issues/README.md`, `docs/README.md`, and `AGENTS.md` updated?
   - **Invariants & CLI Design**: Are command boundaries (`apply` vs `init`) and architectural rules respected?
   - **Media & Demo Verification**: If reels, WebM files, or UI screenshots were produced, has explicit user confirmation been obtained before publishing?
   - **Code Cleanliness**: Is the code token-efficient, idiomatic, and minimal?
   - **Root Cause vs. Symptom**: For defensive/robustness fixes, ask whether the unexpected input's *source* can be fixed instead; verify any upstream fix against the real tool before treating it as done (see `@docs/AgenticLoop.md` Phase 3).
3. Address any review findings before proceeding to commit.
4. Apply this gate to intermediate implementation commits as well as the final sprint commit. After a larger work item is committed and its learnings documented, complete the developer compaction checkpoint from Phase 2; return to Phase 2 if more development remains.

### Phase 4: Process & Subagent Hygiene (Compact, Park & Drain)
1. Inspect running background tasks using the available task/session tools.
2. Explicitly terminate completed, idle, or zombie background jobs, schedule timers, and watch subprocesses. Drain outstanding delegated work before parking sessions.
3. Preserve the reusable advisor and named developer sessions for later reuse; do not blanket-kill them. Confirm the advisor's post-last-ticket compaction and each developer's post-larger-item compaction completed. If a session received additional work afterward, persist its conclusions and have the orchestrator explicitly compact it again as the final call before parking. End disposable reviewer sessions with targeted cleanup.
4. Ensure no background processes are left running unmonitored. A parked reusable session retains its identity without an active job or polling loop.
5. Use actual supported session-compaction operations, not invented commands or self-compaction prompts. If explicit compaction is unavailable or fails, report the affected session and limitation; do not claim it is compacted or ready for clean reuse. Same-session reuse is intended to retain useful cache while clearing ticket context, but cache retention, expiry, and billing depend on the backend and cannot be guaranteed.

### Phase 5: Agentic Flow Quality Retrospective (Feedback & Tracker Sync)
1. Record session flow learnings, tooling friction, or agent harness feedback:
   - Add retrospective notes to `docs/feedback/<YYYY-MM-DD>-<topic>.md` or run `/story` if a major case study was produced.
2. Synchronize the issue tracker:
   - Update issue status in `issues/*.md` (e.g. `Status: Closed`).
   - Update `issues/README.md` table.
   - Run `harnez status` to ensure all issue statuses are clean and validated.
   - When closing a ticket, check the whole file for multiple status-bearing fields and update all (see `@docs/AgenticLoop.md` Phase 5, "Single Status field").
3. Present a clear, concise summary of completed tickets and accomplishments to the user.
