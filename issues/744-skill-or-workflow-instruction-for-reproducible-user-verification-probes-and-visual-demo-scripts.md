# 744 — Skill or workflow instruction for reproducible user verification probes and visual demo scripts

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [AgenticLoop.md](../docs/AgenticLoop.md), [Canary.md](../docs/Canary.md), [RenderingBugPlaybook.md](../../cati/docs/RenderingBugPlaybook.md), #700, #706, #713

---

## 1. Problem & Motivation
When agents or developers complete complex fixes or features (particularly visual rendering, TUI behaviour, data-flow transformations, or performance fast paths), unit test suites verify correctness programmatically, but human users frequently request immediate, concrete visual/interactive proof in their own terminal environment.

In recent work (e.g. in `cati` issues #092 and #094), adding dedicated, lightweight, checked-in verification scripts (such as `scripts/quadblock_fastpath_demo.go` and `scripts/reconstruct_demo.go`) proved to be an exceptionally effective, repeatable method for users to run a single command and visually verify the exact behavior, parity, or pixel reconstruction.

We should institutionalize this practice across Harnez-managed projects: either as a dedicated Harnez skill or as a standard instruction in the core agentic workflow.

## 2. Technical Specification / Options
1. **Option A: Dedicated Skill (e.g., `/probe` or `/verify-probe`)**:
   - Provide a specialized skill that instructs agents how to build minimal, focused, runnable visual probes/demo scripts comparing before/after or fast/reference outputs.
   - Includes standard templates for CLI/TUI side-by-side output and clear exit assertions.
2. **Option B: General Workflow Instruction in Core Agentic Loop**:
   - Update `AgenticLoop.md` (Phase 4 Hygiene / Delivery) and `.harnez/rules/Tools.md` / `Local.md` to require/recommend a user-facing interactive verification command for any visual, format, or TUI ticket.
   - Complements the "Canary-first" and "Rendering Bug Playbook" guidelines.
3. **Option C: Hybrid Approach**:
   - Core rule mandates providing a user-executable verification command/probe upon delivery.
   - A companion skill assists in generating structured visual comparison cards or probe scripts when prompted.

## 3. Implementation & Verification Plan
**Goal**: `/goal Evaluate skill vs general workflow instruction options for user verification probes, design the chosen skill/instruction, and document/ship the convention in Harnez, or stop and report when blocked on a user decision or denied permission.`

1. Audit existing Harnez workflow skills and evergreen docs (`docs/skills/`, `docs/AgenticLoop.md`, `.harnez/rules/`).
2. Design and draft the chosen skill or workflow instruction.
3. Test the instruction on sample workflows across Harnez and consumer repos (e.g. `cati`, `loom`).
4. Commit the documentation and index updates in `harnez`.
