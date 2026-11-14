# 1223 — Generated Git hint says work on the default branch in a protected-main team repo

**Status**: Closed — 146c6c06 (amended): mode-neutral Git hint in config.yaml and AGENTS.md; team-mode init test; live-verified with codex:luna:low
**Closed-By**: session 801d927d-19ac-46f0-aece-6142997b71d9
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

## Decision
- Use the static mode-neutral hint `"conventional commits; branch rules follow Repo Setup; don't push unless asked"` in `config.yaml` rather than coupling language-hint rendering to repo mode (`Repo Setup` already encodes `solo` vs `team`/`fork` workflow). Simple, zero-coupling, reversible.

## Pre-Work (Scout findings)
- `config.yaml:987`: update `agents_md.languages.git.hint` from `"conventional commits, work on the default branch, don't push unless asked"` to `"conventional commits; branch rules follow Repo Setup; don't push unless asked"`.
- `internal/claude/apply.go:145` (`buildLangConventions`) & `internal/claude/init_phases.go:74`: confirmed Repo Setup renders separately from language hints.
- `cmd/harnez/init_gitmode_test.go:38-64` (`TestInitRepoModeWritesRepoYAML`, lines 56–57): add assertion that `--repo-mode team --protected-main` `AGENTS.md` contains the new Git hint and does **not** contain `"work on the default branch"`. Also check `internal/claude/` tests in case any pin the old hint string.
- After editing `config.yaml`, run `make install` and `harnez init -d .`, verify only `AGENTS.md` (and any intentional `config.yaml` copy updates; `git checkout --` unrelated rewrites per `AGENTS.md`) changed, and commit.
- Tests: `go test -race -count=1 ./internal/claude ./cmd/harnez`.

**/goal** Replace the contradicting Git hint in `config.yaml`, add the team-mode regression test, regenerate `AGENTS.md` via `harnez init -d .`, and pass `go test ./internal/claude ./cmd/harnez`, or stop and report when blocked on a user decision or denied permission.
