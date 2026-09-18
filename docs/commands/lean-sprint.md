# Lean Fresh-Handoff Sprint

Orchestrate a lean, fast-path agentic handoff for scoped development tasks and day-to-day tickets.

Reference Practice: `@docs/AgenticLoop.md`

## Role Contract — Direct Execution

You, the agent that received this invocation, are the Host Orchestrator for this task.
Run the complete lean workflow below yourself, in this session: goal handoff, autonomous
execution with self-verification, confidence-gated review, calibrated friction reporting,
and teardown with status sync. Do not delegate the lean-sprint workflow itself to another
orchestrator. The dev subagent in step 1 is a worker you dispatch and whose result you
review — it is not a replacement orchestrator. This is not a choice: direct invocation
always means inline execution. To make a different agent run the lean sprint instead,
the user invokes the `lean-sprinter` delegator, not this skill.

---

## Invocation Syntax

- `/lean-sprint <ticket-numbers>` (e.g. `/lean-sprint 064` or `/lean-sprint 042, 043`)
- `/lean-sprint "<scoped-task-or-bug-description>"`

---

## Lean Fresh-Handoff Workflow

For focused, well-defined tasks, bypass the full 5-phase ceremony in favor of a fast-path execution loop:

### 1. Clean Goal Handoff
- You, as Host Orchestrator, spawn a fresh dev subagent with a single, clear objective, and retain ownership of the workflow.
- Provide:
  - Concise problem statement & target ticket/spec references.
  - Concrete target files/packages and acceptance/verification criteria.
- **Trust the Base Framework**: Do not duplicate or micromanage standard workspace rules, tool descriptions, or formatting conventions already present in the base system prompt.
- **Context & Reading Discipline**: When the task requires inspecting or exploring medium/large source files (>100 lines or multiple slices), instruct the subagent to run `harnez read -I <file>` (visual PNG context card) or `harnez read -L <range>` / `harnez read -n` via command execution instead of issuing repeated native file view calls.
- **Stay Responsive**: Dispatching the subagent must not block the main chat. Report the handoff and return control to the user, or continue only with non-overlapping local work. Do not wait for the subagent unless the user explicitly asks or integration is immediately blocked on its result.

### 2. Autonomous Execution & Self-Verification
- The dev subagent executes the implementation autonomously and reports back to you.
- **Token-Bounded Inspection**: Avoid repetitive built-in file view slices on large files; use `harnez read -I` or `harnez read -L <range>` to inspect code.
- Enforce strict self-verification using repo-native commands (`go test ./...`, `make test`, `make check`, canary probes) — real assertions, not just a clean exit code.
- For defect-shaped tasks (bug/timing/race): establish a concrete reproduction baseline *before* the fix, and verify against that, not just green tests (see `@docs/AgenticLoop.md` Phase 2, "Repro-before-fix").

### 3. Confidence-Gated Inline Review & Milestone-Boundary Commits
- If automated tests pass cleanly and confidence is high (routine bug fix, small feature, internal refactor), skip spawning an independent reviewer subagent.
- The Host Orchestrator performs a rapid inline diff review, including whether a defensive/robustness fix could instead fix its root cause (see `@docs/AgenticLoop.md` Phase 3, "Root Cause vs. Symptom").
- **The "What Would I Have Done Differently?" Check**: The Host Orchestrator explicitly evaluates: *Did the subagent introduce subtle edge-case omissions, performance overheads, or future architectural debt?*
  - *Blocking defects*: Fix or request an immediate correction before advancing.
  - *Non-blocking nuances*: Immediately buffer into the dedicated **Nuance Refinement Ticket** (`#XXX-refinements`) rather than disrupting subagent momentum with iterative micro round-trips.
- **Milestone-Boundary Atomic Commits**: For multi-milestone sprints, the Host Orchestrator (or subagent) **must immediately commit** each verified milestone upon passing tests and inline review (`git commit -m "feat/fix(...): ... (issue XXX MX)"`). Never carry uncommitted working tree diffs across milestone transitions or session resumptions.
- **Adaptive Interception Gate**: If accumulated nuances cross an architectural threshold that impacts upcoming milestones, intercept to sweep them before proceeding; otherwise resolve them in the sprint consolidation phase.
- Escalate to a full Phase 3 Reviewer agent only if:
  - There is cross-subsystem blast radius or architectural ambiguity.
  - Automated tests cannot fully cover runtime behavioral contracts.
  - The dev agent expressed uncertainty or encountered unexpected regressions.

### 4. Calibrated Friction Reporting
- Capture genuine tooling, environment, or sandbox friction **only** after non-trivial sessions where real hurdles occurred.
- Do not emit repetitive boilerplate or trivial environment noise on routine, fast iterations.
- If authentic friction is discovered, record it in `docs/feedback/` or append to related tickets.

### 5. Teardown & Status Sync
- Terminate the dev subagent once its work is reviewed, using the harness's actual subagent lifecycle control (not an invented command name); drain any other background jobs or timers.
- Update ticket status in `issues/*.md` — check the whole file for more than one status-bearing field (see `@docs/AgenticLoop.md` Phase 5, "Single Status field") — and refresh `issues/README.md`.
- Verify with `harnez status`; run `harnez index` instead of hand-editing `issues/README.md` or the studies table.
