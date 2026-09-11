# 312 — Deduplicate global CLAUDE.md sections against the new local AGENTS.md managed block

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics / Docs Pipeline
**Related**: [Issue 311](311-single-managed-section-for-local-agents-md-synced-across-every-project.md), `config.yaml` (`agents_md.global.sections`, `agents_md.local.sections`)

---

## 1. Problem & Motivation

Issue 311 added a single managed "Harnez Managed Conventions" section to every
project's local `AGENTS.md`, containing universal rules including "Editing
Discipline" and "Issue Tracker Discovery". That de-duplicated the copies that
existed between `docs/templates/AGENTS.md` and hand-authored project files.

It did **not** touch `config.yaml`'s `agents_md.global.sections` ("Instructions
Hierarchy" section, `config.yaml` around line 358-380), which independently
hand-authors its own "Editing Discipline" one-liner and a subset of "Issue Tracker
Discovery" content, generating `~/.claude/CLAUDE.md` (loaded into every session
globally, on top of whatever the project-local `AGENTS.md` also loads).

The two copies are no longer byte-identical — issue 311's local version has more
detail (e.g. `harnez issues new`/`harnez index` examples) and different wording,
so every session now loads two different phrasings of "prefer structured patch
tools" and two different subsets of `harnez find`/`harnez issues` usage. This is
exactly the kind of duplication earlier discussed as low-value repetition (not the
"reinforcement" kind), since both files load into the same session simultaneously.

## 2. Scope & Technical Requirements

- Decide: either drop the "Editing Discipline" content from
  `agents_md.global.sections`' "Instructions Hierarchy" section entirely (since
  every project with `agents_md.local.sections` applied already carries it), or
  replace it with a one-line pointer.
- Same decision for the "Issue Tracker Discovery" overlap.
- Global `CLAUDE.md` still needs to work standalone for contexts where a project's
  local `AGENTS.md` doesn't have the new managed block yet (not every project will
  have run `init` again immediately) — weigh whether a pointer is safe there, or
  whether the global copy should stay self-contained and only the wording should be
  unified (single source of truth authored once, referenced twice) rather than
  removed outright.
- Do not change unrelated global sections (Tool Feedback Protocol, Agent-Filed
  Feedback) — out of scope.

## 3. Acceptance Criteria

- [ ] No two sections across `agents_md.global.sections` and
      `agents_md.local.sections` restate the same rule with different wording.
- [ ] Global `~/.claude/CLAUDE.md` remains correct and useful for a project that has
      not yet picked up the new local managed block.
- [ ] `make lint`, `make check`, and existing global/local section tests pass
      unchanged in intent (update only what this ticket's content change requires).

## 4. Notes

Surfaced during issue 311's review pass — not a blocker for that ticket (its own
acceptance criteria explicitly left this as an implementation call), filed
separately to keep 311's diff scoped.
