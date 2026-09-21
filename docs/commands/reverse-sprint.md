# Reverse Sprint — Low-Cost Dev Lead with On-Demand Review & Escalation

Orchestrate a bottom-up, cost-efficient sprint where a **low-cost developer agent** leads implementation, auto-compacts frequently, and selectively calls reviewer or advisor subagents on demand.

Reference Practice: `@docs/AgenticLoop.md`

## Model Selection Matrix

| Role | Permitted Models | Usage Trigger | Context & Exploration Rules |
|---|---|---|---|
| **Coder / Main Session** | `luna:low`, `luna:medium`, `haiku`, `gemini3.7flash:low` | Primary executor (all coding, tests, commits) | Leads session; compacts every 100–150k tokens |
| **Reviewer Subagent** | `sol:low`, `sonnet:low`, `gemini3.8flash:low` | After milestone completion or before commit | Diff-first inspection (`git diff HEAD~1`, test results) |
| **Advisor Subagent** | `astra:low`, `opus:low`, `gemini3.8flash:med` | **Use only when stuck** or solving challenging problems | **Strict limited context**: direct file pointers, exact lines, zero deep crawling |

---

## Role Contract — Low-Cost Developer Lead

You are the lead developer driving this sprint in a low-cost, token-efficient tier.
You execute all coding, testing, and ticket management directly while maintaining strict context and cost discipline:

1. **Direct Implementation & TDD**:
   - You write code, author tests (`go test ./...`, `make test`), and fix bugs directly.
   - Maintain codebase stability and compilability at every step.
   - For bug fixes, establish a reproduction baseline before applying fixes.

2. **Frequent Auto-Compaction (100–150k Token Threshold)**:
   - To avoid context degradation and memory loss on low-tier models, **proactively trigger session compaction every 100–150k tokens** or immediately upon completing a milestone.
   - Persist critical learnings, ticket status, and decisions to ticket/doc files *before* compacting.

3. **On-Demand Milestone Code Reviews**:
   - After completing a milestone and before committing, spawn or reuse a **Reviewer subagent** (`sol:low`, `sonnet:low`, or `gemini3.8flash:low`).
   - Supply only the commit diff (`git log -n 1 --stat`, `git diff HEAD~1`) and test output.
   - Address any identified regressions, missing test assertions, or ambient leaks before advancing.

4. **Advisor Escalation — Strict Context Bounding (Use Only When Stuck)**:
   - When encountering architectural ambiguity, tough edge cases, or blocking bugs, consult an **Advisor subagent** (`astra:low`, `opus:low`, or `gemini3.8flash:med`).
   - **Crucial Cost Guardrail**: Provide *strictly limited context* — direct file pointers, exact line ranges (`path/to/file.ext#L20-L50`), and concrete questions.
   - Explicitly instruct the advisor **NOT to perform whole-repo exploratory browsing or multi-file deep scans** to avoid exhausting frontier token budgets (especially on Astra/Opus).

---

## Invocation Syntax

- `/reverse-sprint <ticket-numbers>` (e.g. `/reverse-sprint 042` or `/reverse-sprint 108, 109`)
- `/reverse-sprint "<scoped-task-or-bug-description>"`

---

## Reverse Sprint Workflow

### 1. Goal & Live Codebase Inspection
- Read the target ticket in `issues/` and identify the `/goal` and acceptance criteria.
- Because tickets may sit in the backlog over time, verify current code status and recent commits before writing code.
- Formulate a brief, minimal implementation plan. Avoid creating elaborate milestone breakdowns unless the task genuinely requires multi-stage handoffs.

### 2. Implementation & Test Verification
- Implement the scoped changes using Test-Driven Development (TDD).
- Run repo-native verification commands (`go test ./...`, `make test`, `make check`).
- Ensure all tests pass with robust assertions.

### 3. Milestone Review Gate (Reviewer Tier)
- Invoke a reviewer subagent (e.g. `harnez agent start --role reviewer --model sol -d <dir> "Review diff HEAD~1 against ticket criteria"` or `gemini3.8flash:low`).
- Provide diff summary and test results. Note the emitted Reconnect Banner.
- Reviewer checks test assertion rigor, regression risks, and invariant compliance.
- Once green, commit the milestone: `git commit -m "feat/fix(...): ... (issue XXX)"`.

### 4. Proactive Auto-Compaction
- Check current context token usage.
- If approaching 100–150k tokens or transitioning between major milestones, persist working notes and trigger session compaction via `harnez agent compact --name <session_id>` or automated runner compaction.

### 5. Advisor Escalation (If Stuck / Challenging Blockers)
- If stuck on a complex architectural decision or subtle bug:
  - Isolate the problem to specific files and line numbers.
  - Dispatch a single advisor subagent:
    ```bash
    harnez agent start --role advisor --model astra "Review lines 45-80 of pkg/service/handler.go for the race condition described below. Do not explore unrelated files."
    ```
  - Integrate advisor recommendations and resume direct execution.

### 6. Teardown & Status Sync
- Terminate all child reviewer/advisor subagents (`harnez agent delete --name <session_id>` or `harnez agent stop --all` / `manage_subagents kill`).
- Update ticket status in `issues/*.md` (e.g. `Status: Closed — resolved`) and resync tracker (`harnez index`).
- Commit the ticket update and tracker sync immediately.
- Summarize delivered results and verification status.
