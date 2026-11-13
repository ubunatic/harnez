# 1223 — Generated Git hint says work on the default branch in a protected-main team repo

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1221, 1039

---

## 1. Problem & Motivation

The doc list that `harnez init` writes into `AGENTS.md` carries a fixed Git hint: "conventional
commits, work on the default branch, don't push unless asked". In a repository with
`git.mode: team` and `protected_main`, the Repo Setup block a few lines above says the opposite:
work on a local topic branch, `main` is protected. A `luna:high` session in 1221 reported the
contradiction when asked which instructions conflicted.

The hint is one of the few lines every agent has in context, so it should not contradict the
repository's recorded mode.

## 2. Technical Specification / Findings

- Source: `config.yaml`, `agents_md.languages.git.hint` (a static string).
- `docs/lang/Git.md` itself is correct: default branch for `solo`, topic branch and PR for
  `team`/`fork`.
- The Repo Setup block is already rendered per mode (1039).

## 3. Implementation & Verification Plan

- Make the hint mode-neutral, for example "conventional commits; branch rules follow Repo Setup;
  don't push unless asked", or render it from `git.mode` like Repo Setup.
- Test: an init in `team` mode must not produce "work on the default branch" in `AGENTS.md`.
- Run `harnez init -d .` here and commit the regenerated `AGENTS.md`.
