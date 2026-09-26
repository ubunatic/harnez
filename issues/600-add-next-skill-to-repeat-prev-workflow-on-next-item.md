# 600 — Add /next skill to repeat prev workflow on next item

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Skills / Workflow / Automation
**Related**: `docs/commands/`, `docs/commands/lean-sprint.md`, `docs/commands/HarnezStatus.md`, `docs/practices/AgenticLoop.md`

---

## Goal

`/goal`: Add a `/next` skill that allows the agent to inspect the recent session history / tasklist / roadmap and automatically resume or repeat the previous workflow (e.g. `/lean-sprint`, review, or filing) on the next ticket, milestone, or task.

## 1. Problem & Motivation

When executing a sequence of tasks (such as consecutive lean sprints, milestone tickets on a roadmap, or items on an agent tasklist), the human developer frequently asks "what's next?" or wants to proceed with the same orchestration workflow on the subsequent item.

Currently, the user must repeatedly type out the full command or issue number (e.g. `/lean-sprint <N> use luna:med as dev and terra as reviewer`). A dedicated `/next` skill enables seamless chaining by:
1. Detecting the previous workflow parameters (e.g. `/lean-sprint`, model preferences like `luna:med` / `terra:med`).
2. Discovering the next candidate item from the roadmap (`docs/Roadmap.md`), open issue tracker (`harnez find issues -a status:open`), or active sprint plan.
3. Proposing or immediately triggering the next workflow execution in accordance with the established sprint invariants.

## 2. Technical Specification & Skill Contract

### Skill Attributes
- **Name**: `next` / `/next`
- **Mode**: User-invocable workflow skill.
- **Location**: `docs/commands/Next.md` (and registered in `config.yaml` / skill templates for `apply` / `init`).

### Workflow Logic
1. **Context Extraction**:
   - Inspect conversation history / recent instructions to identify the most recently used workflow pattern (e.g., `/lean-sprint`, `/issue`, `/review`) and associated flags/models.
2. **Target Discovery**:
   - Query the next actionable open issue (`harnez find issues -a status:open`), next milestone on the roadmap, or next subtask.
3. **Execution / Recommendation**:
   - If unambiguous or explicitly commanded with auto-advance, execute the workflow on the next item.
   - Present a concise confirmation showing the selected item and workflow parameters.

## 3. Implementation Plan

- **M1**: Author `docs/commands/Next.md` outlining the skill rules, workflow discovery, and invocation semantics.
- **M2**: Register `/next` in `config.yaml` / skill manifest and update skill registration tests in `internal/claude`.
- **M3**: Verify with test suite (`make test-q1`) and `make install`.

## 4. Acceptance Criteria

- `/next` skill documentation and template installed and registered.
- Skill clearly instructs the model how to infer previous workflow parameters and determine the next ticket/milestone without losing context.
- Unit tests for skill registries pass.
