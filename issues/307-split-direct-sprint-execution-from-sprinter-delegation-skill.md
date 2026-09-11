# 307 — Deterministic direct execution and delegation for sprint workflows

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [Issue 039](039-agentic-loop-practices-and-sprint-command.md), [Issue 288](288-add-fresh-codex-skill-lean-fresh-handoff-sprint-using-codex-cli-instead-of-a-claude-subagent.md), [Commands & Skills Pipeline](../docs/CommandsPipeline.md), [Agentic Loop Practices](../docs/AgenticLoop.md)

---

## 1. Problem & Motivation

The invocation surface does not deterministically tell the receiving agent whether it owns sprint orchestration or must spawn a new orchestrator. In practice, agents have inconsistently chosen inline execution versus spawning another agent, producing unpredictable orchestration depth, responsiveness, lifecycle handling, and verification. The model must never freely choose between those roles.

This issue separates both workflow pairs:

- `/sprint` is the direct full-workflow entry point. The current user-facing agent executes its complete five-phase workflow.
- A brief `sprinter` delegator tells a user-selected-model subagent to read the installed `sprint` skill completely and execute the supplied task under that workflow.
- `/fresh-sprint` is likewise the direct lean-workflow entry point. The current user-facing agent executes its complete fresh-sprint workflow.
- A brief fresh-sprint delegator tells a user-selected-model subagent to read the installed `fresh-sprint` skill completely and execute the supplied task under that workflow.

The direct skills and delegators must make this role distinction explicit: direct invocation means inline execution by the receiving agent; delegator invocation means spawning exactly the selected subagent as the new orchestrator. Neither path may leave that choice to model discretion. Preserve the current sprint workflow’s sequential reusable advisor → reusable developers → review → hygiene → retro structure.

## 2. Scope & Technical Requirements

- Update `commands/sprint.md` so `/sprint` explicitly owns and executes the full five-phase workflow in the receiving user-facing agent, without spawning a separate sprint orchestrator for the overall task.
- Update `commands/fresh-sprint.md` so `/fresh-sprint` explicitly owns and executes its complete lean workflow in the receiving user-facing agent, without silently converting direct invocation into delegation. Preserve its existing handoff, autonomous execution/self-verification, confidence-gated review, friction reporting, and teardown/status-sync phases as the direct workflow contract.
- Add a concise `sprinter` source skill/prompt. It must pass through the supplied task context, require the selected subagent to read the installed `sprint` skill completely before acting, and require that subagent to execute the task by following `sprint`. It must not reproduce or reinterpret the five-phase playbook, select a model on the user’s behalf, or permit inline execution by the delegating receiver.
- Add a clearly named companion for the fresh workflow. `fresh-sprinter` is the preferred name because that installed Codex path already exists; if source/config conventions establish a different name, choose one bounded alternative and use it consistently everywhere. Its brief prompt must require the selected subagent to read the installed `fresh-sprint` skill completely and execute the task by following it, with no free choice between inline execution and delegation.
- Register both delegators and both direct skills through the established `config.yaml` installation surfaces. Keep command-versus-skill boundaries explicit: direct `/sprint` and `/fresh-sprint` commands remain direct workflow entry points, while the two delegators are registered skills unless repository evidence requires another surface.
- Regenerate/install the configured artifacts through the normal pipeline, including the Codex paths `/home/uwe/.codex/skills/sprint/SKILL.md`, `/home/uwe/.codex/skills/fresh-sprint/SKILL.md`, `/home/uwe/.codex/skills/sprinter/SKILL.md`, and the final chosen fresh delegator path (expected `/home/uwe/.codex/skills/fresh-sprinter/SKILL.md`). Do not hand-edit installed copies.
- Do not change the workflow into a new orchestration engine, alter unrelated advisor/reviewer/telemetry behavior, or remove the sequential reusable advisor/developer model.

## 3. Acceptance Criteria

- [x] `/sprint` unambiguously instructs the receiving user-facing agent to execute the full five phases inline; it does not instruct that agent to spawn another sprint orchestrator.
- [x] `/fresh-sprint` unambiguously instructs the receiving user-facing agent to execute its complete lean workflow inline; it does not leave inline-versus-delegated execution to model choice.
- [x] The `/sprint` workflow still requires sequential reusable advisor discovery, sequential reusable developers, an independent pre-commit review gate, process/subagent hygiene, and a flow-quality retrospective.
- [x] A registered `sprinter` delegator is brief and self-contained, passes the given task context to a user-selected-model subagent, requires a complete read of the installed `sprint` skill, and makes that subagent the sprint orchestrator.
- [x] A registered fresh-sprint delegator (preferably `fresh-sprinter`, or one documented bounded alternative) is equally brief and deterministic, requires a complete read of the installed `fresh-sprint` skill, and makes the selected subagent the fresh-sprint orchestrator.
- [x] Both delegators explicitly require delegation, while both direct skills explicitly require inline execution; no prompt says or implies that the receiving model may choose the role.
- [x] Neither delegator duplicates or reinterprets its full workflow, hard-codes a model, or adds a second orchestration policy that can drift from the corresponding direct skill.
- [x] `config.yaml` registers all four skills/paths correctly without creating an unintended command or omitting a configured target; the final naming choice is consistent in source, config, generated output, and documentation.
- [x] Normal apply/generation refreshes all four installed Codex skill artifacts, including the direct skills and both brief delegators, and verification confirms their role language is current and idempotent.
- [x] No unrelated product, skill, command, or generated files are modified by the ticket’s implementation beyond the files and generated targets intentionally covered by this scope.

## 5. Implementation Notes

- Delegator sources live in `docs/commands/` (`Sprinter.md`, `FreshSprinter.md`), registered only under `skills:` in `config.yaml` — not under `commands/`, since `scripts/lint.sh` forces every `commands/*.md` file to also register a slash command, which would have created an unintended command.
- Correction to §1: the `fresh-sprinter` Codex path did **not** already exist prior to this ticket (`~/.codex/skills/` had no `fresh-sprinter` entry). The name was still adopted for symmetry with `sprinter`.
- Registering the two new skills fans out to 8 installed directories (Gemini, Codex, Claude, Prime skill roots × 2 skills), not just the 4 Codex paths named in scope — this is normal pipeline behavior (`internal/claude/apply.go` `skillTargets`), not scope creep.
- Verified via `make lint`, `make check`, `make install`, `make apply` (twice, second run reported "No changes" — idempotent), `make status`, `scripts/smoke-test.sh`, and `harnez status`.

## 4. Migration & Verification Plan

1. Inspect the existing command/skill pipeline and migrate the source prompts and `config.yaml` registration together. Remove stale delegation wording from the direct skills and add both delegator registrations; resolve the fresh delegator name from repository evidence, preferring the existing `fresh-sprinter` installed path.
2. Run focused configuration/skill generation checks and apply/status checks. Inspect `/home/uwe/.codex/skills/{sprint,fresh-sprint,sprinter,<fresh-delegator>}/SKILL.md` and confirm source/config/generated names agree.
3. Search source and installed prompts for old ambiguous role language. Verify direct prompts say the receiving agent executes inline, delegators say the selected subagent executes after reading the corresponding full skill, and neither pair permits discretionary role selection.
4. Re-run apply/status or equivalent idempotency checks, then relevant repository tests/check targets. Inspect `git diff` and `git status` to confirm the implementation is scoped and no installed artifact was hand-edited.
