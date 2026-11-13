# 1225 — Add /harnez-cleanup skill and verify it with a terra:high run on one skill

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1221, 1224

---

## 1. Problem & Motivation

1221 trimmed the core agent docs by hand: measure context cost, shrink, retest on cheap models,
file follow-ups. The owner wants that as a repeatable skill, `/harnez-cleanup <scope>`, written
by the same rules it teaches: assume capable agents, no format prescriptions, only proven
`harnez` commands.

## 2. Technical Specification / Findings

- Source: `docs/commands/HarnezCleanup.md` (76 lines, 3764 bytes), registered in `config.yaml`
  as `harnez-cleanup`, `user-invocable-only`.
- Scope is prose: repository rules by default, named skills, or a full sweep.
- Every `harnez` command in the skill ran in the 1221 session or while writing the skill:
  `harnez agent start --name --role developer --model -f`, `harnez agent resume --name --`,
  `harnez agent models`, `harnez agent delete --force --name`, `harnez issues new -d .`.
- Role choice for a commanded run: `orchestrator` (`spec/agent.yaml`, `spawns: [developer,
  reviewer, advisor]`). It may not edit, so the rewrite goes to one developer helper. No new
  role; a role that both edits and spawns would break the one-writer design.

## 3. Implementation & Verification Plan

- `make test-q1` on the skill and config change.
- Verification run: a `terra:high` orchestrator follows the skill for one existing skill,
  `harnez-init` (`docs/commands/HarnezInit.md`, 59 lines, 3818 bytes, with a test in
  `internal/claude/harnez_init_skill_test.go`).

### Task for the verification run

Follow `docs/commands/HarnezCleanup.md` with scope "the harnez-init skill".

- Budget: at most four test sessions (one cheap model at `low` and `high`, before and after),
  plus one developer helper for the rewrite.
- Test sessions read the skill source by path (`docs/commands/HarnezInit.md`), so no reinstall
  is needed between runs. They work in scratch repositories outside this checkout and never run
  `harnez init` or commit in this repository.
- The developer helper edits `docs/commands/HarnezInit.md` only, on the checked-out branch, and
  commits. No push, no `harnez apply`.
- Skip the study; file no tickets. Write under "Verification run" below: sizes before and
  after, the checks and score per session, what the sessions said they set aside, and every
  place where the cleanup skill was unclear, wrong or missing a step.

### Verification run

(to be written by the run)
