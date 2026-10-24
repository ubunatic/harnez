# 901 — init keeps issues.tracker none after a harnez tracker appears in a new repo

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 853 (claimed the first `issues new` adopts `tracker: harnez`), 899 (new-project skill hit it in the dry run)

---

/goal A fresh repo that gets its first harnez ticket ends with `issues.tracker: harnez` in `.harnez/repo.yaml`
after the next `harnez init`, covered by a regression test; or stop and report when the fix needs an owner
decision on the opt-in rule from 853. Circuit breaker: on a loop or no progress, clear the goal and write a
failure report here.

## Repro (2026-10-24, scratch dir)

1. `git init`, `harnez init --quota-1 --variant lite --docs make,golang,bash,manpages` → `tracker: none`
   (no `issues/` yet, correct).
2. `harnez issues new -d . "bootstrap epic"` → prints "local mode", writes `issues/001-…` and a managed README.
3. `harnez init -d .` (also with `--issues-git`) → still `tracker: none` (`merge_driver: true` with
   `--issues-git`, an inconsistent pair).
4. `rm .harnez/repo.yaml && harnez init -d .` → `tracker: harnez`.

`detectRepoConfig` (internal/claude/repo_config.go) only runs when `repo.yaml` is absent, so a stored
`none` never upgrades. Workaround in the new-project skill: step 4 of docs/commands/new-project.md.
