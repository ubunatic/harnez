# 752 — Add two-agent lean-sprint variant and rename legacy single-dev loop to lean-sprint-v1

**Status**: Closed — Implemented two-agent lean-sprint variant and preserved legacy single-dev loop as lean-sprint-v1 in commit 6c85bda9
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/465-consider-adopting-loom-s-lean-sprint-field-notes-into-docs-practices-agenticloop.md, issues/523-lean-sprint-host-runs-live-harnez-agent-checks-leaf-workers-cannot.md

---

## 1. Problem & Motivation

The standard `/lean-sprint` workflow previously assumed a single developer subagent interacting with the host orchestrator. In practice, tasks often require two distinct subagent capabilities:
1. A very cheap, token-efficient search/research agent (`luna:low` / `luna-search`) to perform codebase inquiries, explore APIs, read specs, and answer questions.
2. A focused developer agent (`luna:high` or `sol:low`) to execute TDD, write code, run verification tests, and produce commits.

To preserve compatibility and allow side-by-side performance comparisons, the legacy single-dev loop must be preserved as `/lean-sprint-v1`, while `/lean-sprint` is updated to support the two-agent lean sprint pattern (cheap searcher + inexpensive dev worker).

## 2. Technical Specification / Findings

- **Existing Skills & Docs**:
  - Global skill paths: `~/.claude/skills/lean-sprint/SKILL.md` and `~/.gemini/skills/lean-sprint/SKILL.md`.
  - Repo docs: [`docs/commands/lean-sprint.md`](file:///home/uwe/projects/harnez/docs/commands/lean-sprint.md) and [`docs/practices/AgenticLoop.md`](file:///home/uwe/projects/harnez/docs/practices/AgenticLoop.md).
- **Two-Agent Structure (New `/lean-sprint`)**:
  - Host Orchestrator maintains strict zero-coding and diff-first review invariants.
  - Searcher (`luna:low` / `luna-search`): Used for read-only audits, questions, and discovery without polluting host context.
  - Developer (`luna:high` or `sol:low`): Dispatched for sequential milestone implementation and TDD.
  - Sidetask documentation: Emits/maintains running sprint study report in `docs/studies/` tracking token efficiency and progression.
- **Legacy Preservation (`/lean-sprint-v1`)**:
  - Duplicate current `lean-sprint` skill as `lean-sprint-v1`.
  - Ensure users can invoke `/lean-sprint-v1` to run the baseline single-dev workflow.

## 3. Implementation & Verification Plan

### Goal
/goal Create `/lean-sprint-v1` from the legacy lean-sprint skill/docs and update `/lean-sprint` with the two-agent (cheap searcher + inexpensive dev) workflow, or stop and report when blocked on user input or denied permission.

### Milestones
- **M1: Legacy Preservation (`lean-sprint-v1`)**:
  - Copy and register `lean-sprint-v1` skill across `~/.claude/skills/` and `~/.gemini/skills/` (and repository command docs if applicable).
- **M2: Update `/lean-sprint`**:
  - Update `lean-sprint` skill definitions and documentation to specify the two-agent division of labor (cheap searcher + inexpensive dev) and running study report requirement.
- **M3: Verification**:
  - Verify that both `/lean-sprint` and `/lean-sprint-v1` are resolvable, pass validation checks, and have synchronized docs.

## 4. Addendum: Prompt Files Over Verbatim Inline Prompts

- **Problem observed**: Long inline prompts passed via `-p "..."` or positional shell arguments suffer from shell interpolation footguns (e.g., unescaped backticks being evaluated by host bash as subcommands).
- **Rule & Guidance**: Dispatches and documentation should strongly emphasize using prompt files (`-f <file>` / task files) rather than long verbatim inline prompts when handing work to subagents or orchestrating complex turns.
