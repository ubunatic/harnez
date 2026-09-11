# Lean Fresh-Handoff Sprint

Orchestrate a lean, fast-path agentic handoff for scoped development tasks and day-to-day tickets.

Reference Practice: `@docs/AgenticLoop.md`

## Role Contract — Direct Execution

You, the agent that received this invocation, are the Host Orchestrator for this task.
Run the complete lean workflow below yourself, in this session: goal handoff, autonomous
execution with self-verification, confidence-gated review, calibrated friction reporting,
and teardown with status sync. Do not delegate the fresh-sprint workflow itself to another
orchestrator. The dev subagent in step 1 is a worker you dispatch and whose result you
review — it is not a replacement orchestrator. This is not a choice: direct invocation
always means inline execution. To make a different agent run the fresh sprint instead,
the user invokes the `fresh-sprinter` delegator, not this skill.

---

## Invocation Syntax

- `/fresh-sprint <ticket-numbers>` (e.g. `/fresh-sprint 064` or `/fresh-sprint 042, 043`)
- `/fresh-sprint "<scoped-task-or-bug-description>"`

---

## Lean Fresh-Handoff Workflow

For focused, well-defined tasks, bypass the full 5-phase ceremony in favor of a fast-path execution loop:

### 1. Clean Goal Handoff
- You, as Host Orchestrator, spawn a fresh dev subagent with a single, clear objective, and retain ownership of the workflow.
- Provide:
  - Concise problem statement & target ticket/spec references.
  - Concrete target files/packages and acceptance/verification criteria.
- **Trust the Base Framework**: Do not duplicate or micromanage standard workspace rules, tool descriptions, or formatting conventions already present in the base system prompt.
- **Stay Responsive**: Dispatching the subagent must not block the main chat. Report the handoff and return control to the user, or continue only with non-overlapping local work. Do not wait for the subagent unless the user explicitly asks or integration is immediately blocked on its result.

### 2. Autonomous Execution & Self-Verification
- The dev subagent executes the implementation autonomously and reports back to you.
- Enforce strict self-verification using repo-native commands (`go test ./...`, `make test`, `make check`, canary probes).
- Must verify passing tests with real assertions before declaring completion.

### 3. Confidence-Gated Inline Review
- If automated tests pass cleanly and confidence is high (routine bug fix, small feature, internal refactor), skip spawning an independent reviewer subagent.
- The Host Orchestrator performs a rapid inline diff review.
- Escalate to a full Phase 3 Reviewer agent only if:
  - There is cross-subsystem blast radius or architectural ambiguity.
  - Automated tests cannot fully cover runtime behavioral contracts.
  - The dev agent expressed uncertainty or encountered unexpected regressions.

### 4. Calibrated Friction Reporting
- Capture genuine tooling, environment, or sandbox friction **only** after non-trivial sessions where real hurdles occurred.
- Do not emit repetitive boilerplate or trivial environment noise on routine, fast iterations.
- If authentic friction is discovered, record it in `docs/feedback/` or append to related tickets.

### 5. Fast Teardown & Status Sync
- Terminate finished subagents immediately (`manage_subagents kill`).
- Ensure no lingering background processes or timers remain.
- Record an `--ok` heartbeat (`harnez rate --ok "<note>" [<ticket>]`) to confirm clean sprint completion in telemetry.
- Update ticket status in `issues/*.md` and `issues/README.md`.
- Verify with `harnez status`; run `harnez index` to regenerate `issues/README.md` and
  `docs/README.md`'s studies table instead of hand-editing rows.
