# 499 — Assess baking the model-aware sprint orchestration approach into skills

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Design
**Related**: `docs/practices/ModelRoles.md`, `docs/ModelAdvisoryEval.md`, `docs/feedback/2026-09-22-lean-sprint-491.md`, `docs/feedback/2026-09-22-lean-sprint-496-488-497.md`, `docs/commands/{lean-sprint,sprint,reverse-sprint}.md`, 485 (quota-aware default model), 445/446 (cost data), 498 (Claude resume), 493 (dispatch mode)

## /goal

A decision, recorded in this ticket, on which of the session's orchestration techniques go into
which skill (existing or new), plus the resulting skill edits or follow-up tickets. After it, a
sprint run by a strong host gets the model-aware behaviour from the skills, not from the host
re-deriving it.

## Context

On 2026-09-22 an Opus host ran two lean sprints (496/488/497, then 491) with a way of working
that no skill describes yet. The only written record is the new practice doc
`docs/practices/ModelRoles.md` and the eval data in `docs/ModelAdvisoryEval.md`. The sprint
skills still treat model choice as "e.g. `--model luna`".

Techniques used, to assess one by one:

1. **Initial model assessment.** Before the sprint, the same read-only advisory prompt went to
   five candidates (luna:low/med, terra:low, haiku, sonnet), and each answer was graded against
   the repo with a fact-check grid. This yielded the role table and found a stale ticket (289).
2. **Assign by capability and cost.** Cheap cross-vendor advisors (terra:low + sonnet),
   developer tier by ticket risk (luna:low for clear tickets, luna:med for interface changes),
   a different-vendor reviewer at the seam, haiku only for mechanical work.
3. **Watch plan usage and react.** `harnez usage` was checked before each milestone, with a
   stop threshold (99% weekly). When Codex hit it, the remaining milestones moved to
   `claude:sonnet`, which worked because the ticket carried all context. Quota is whole points
   only, and `harnez agent delete` wipes per-turn readings.
4. **Strong host as session runner.** A high-tier model (Opus) as orchestrator that writes no
   code, reviews diffs, writes pre-work, verifies invariants itself (such as a golden hash on
   pre-change code, or tracing the CLI path), and checks reviewer "blocking" claims against the
   design. It caught the bugs that the cheap workers' green tests hid.
5. **Read-only plan turn before writing**, and **named items in reports** (the latter is already
   in `AgenticLoop.md` §4).

## Questions to answer

- Which technique fits which home: `lean-sprint`/`sprint`/`reverse-sprint`, `AgenticLoop.md`,
  `ModelRoles.md`, a new skill (e.g. a `/model-eval` for technique 1), or harnez code
  (technique 3 overlaps 485: automatic default-model choice from limits)?
- Should skills name concrete models, or roles resolved through `spec/agent.yaml` so that
  lineup changes don't require skill edits (`docs/Spec.md`)?
- Should skills recommend a high-tier session runner, and what should a skill do when the host
  is itself a cheap model: warn, or narrow its duties?
- How does quota watching work per provider without cost data (445/446) and with whole-point
  resolution? What is the minimum useful check?
- What breaks with the current limits: Claude workers are single-turn inside Claude Code (498),
  and a plan-then-resume needs a working resume.

## Non-goals

- Implementing 485, 445 or 446 here. Reference them when a technique depends on them.
- Freezing the current model lineup into skills.

## Acceptance

- Each of the five techniques has a recorded decision: target (skill, doc, code, or drop) with
  a one-line rationale.
- The agreed skill and doc edits have landed, or are filed as tickets linked here.
- Rerunning a lean sprint from the updated skills alone reproduces the role assignment and the
  quota check, without host improvisation.
