# Lean Fresh-Handoff Sprint

Orchestrate a lean, fast-path agentic handoff for scoped development tasks and day-to-day tickets.

Reference Practice: `@docs/AgenticLoop.md`

---

## Invocation Syntax

- `/fresh-sprint <ticket-numbers>` (e.g. `/fresh-sprint 064` or `/fresh-sprint 042, 043`)
- `/fresh-sprint "<scoped-task-or-bug-description>"`

---

## Lean Fresh-Handoff Workflow

For focused, well-defined tasks, bypass the full 5-phase ceremony in favor of a fast-path execution loop:

### 1. Clean Goal Handoff
- The Host Orchestrator spawns a fresh subagent with a single, clear objective.
- Provide:
  - Concise problem statement & target ticket/spec references.
  - Concrete target files/packages and acceptance/verification criteria.
- **Trust the Base Framework**: Do not duplicate or micromanage standard workspace rules, tool descriptions, or formatting conventions already present in the base system prompt.
- **Stay Responsive**: Dispatching the subagent must not block the main chat. Report the handoff and return control to the user, or continue only with non-overlapping local work. Do not wait for the subagent unless the user explicitly asks or integration is immediately blocked on its result.

### 2. Autonomous Execution & Self-Verification
- The subagent executes the implementation autonomously.
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
- Update ticket status in `issues/*.md` and `issues/README.md`.
- Verify with `harnez status`; run `harnez index` to regenerate `issues/README.md` and
  `docs/README.md`'s studies table instead of hand-editing rows.
