# 731 — Rename docs/commands to docs/skills and turn the last three commands into skills

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: `docs/CommandsPipeline.md`, `config.yaml` (`commands:`, `skills:`, `debloat:`), `embed.go`, issue 726 (found there)

---

## 1. Problem & Motivation

All 28 files in `docs/commands/` are skill sources. Whether a skill is user-only or also
agent-invoked is set in `config.yaml` `debloat:` (`user-invocable-only`). The folder name and parts
of `docs/CommandsPipeline.md` (e.g. "Source reuse", the commands-vs-skills split) still describe
the old model, which misled an agent looking for skill sources in issue 726.

The only remaining `commands:` entries are three inline ones in `config.yaml`: `standup`,
`commit`, `pull` (installed to `~/.claude/commands/`).

## 2. Technical Specification / Findings

References to `docs/commands` today: `embed.go`, `Makefile`, `internal/assess/tracks.go` (+ test),
`internal/subagent/agentspec_test.go`, five `internal/claude/*_skill_test.go`, and docs
`CommandsPipeline.md`, `ExternalSkills.md`, `ModelAdvisoryEval.md`, `OrchestratedAgentFlow.md`.
Re-grep before starting; the list may have grown. Studies under `docs/studies/` are history and
keep the old path.

To decide during the work: whether the `commands:` mechanism (`genCommandContent`, the
`~/.claude/commands/` target) can be removed once the three commands are skills, and what apply
does with already installed `~/.claude/commands/{standup,commit,pull}.md` (clean up as managed
files, like other removed outputs).

## 3. Implementation & Verification Plan

/goal Skill sources live in `docs/skills/`; `standup`, `commit` and `pull` are user-invocable
skills; the commands mechanism is removed or its remaining purpose documented;
`docs/CommandsPipeline.md` describes the skills-only model. Done when tests pass, `make install`
and `harnez apply` install every skill as before, and no stale `~/.claude/commands/` files remain.
Stop and report when blocked on a user decision or denied permission.
