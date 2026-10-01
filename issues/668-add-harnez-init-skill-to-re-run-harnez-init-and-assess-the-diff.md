# 668 — Add /harnez-init skill to re-run harnez init and assess the diff

**Status**: Closed — Implemented /harnez-init skill with guard check on uninitialized repos, init re-run, regression diff assessment, local fallout rules, upstream issue reporting, and commit steps; verified in uninitialized and initialized repositories
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 667 (found by doing this workflow by hand in voxi)

---

## 1. Motivation

Re-syncing a consumer repo is a recurring manual routine: run `harnez init`, read the
diff, fix small local fallout, commit, and file real upstream bugs in harnez. On
2026-10-01 in voxi this routine found 667 (lite IssueTracking doc points to itself).
A skill makes the routine one command.

## 2. Proposal

A bundled `/harnez-init` skill that:

1. **Guard:** checks the repo was already initialized (e.g. `.harnez/` exists or `harnez:begin` markers in AGENTS.md/CLAUDE.md).
   If not, it stops without running anything and asks the user to choose the init
   setup first: lite vs full docs (`--variant lite` vs `--variant full`), with or without quota1 (`--quota-1`), doc set (`--docs <names>`), and repo mode (`-m solo|fork|team`).
   First-time init is a user decision, never the agent's.
2. Runs `harnez init`.
3. Assesses the diff (`git diff`): explains each change in one line, flags regressions
   (broken links, self-references, contradicting rules, lost local edits).
4. Fixes small local fallout that survives the next init; never hand-edits generated
   files that init would overwrite.
5. Files bigger problems as issues in the harnez repo, with the consumer repo and
   diff excerpt as evidence.
6. Commits the init changes in the consumer repo (no push) and reports.

## 3. Resolution & Open Questions

- **Initialization Marker**: The skill checks for `.harnez/` directory in the repository root or `<!-- harnez:begin` markers in `AGENTS.md` / `CLAUDE.md`.
- **Option Persistence**: Handled via explicit flags when specified by user; re-runs preserve existing file structures and managed doc sets.
- **Skill Registration**: Registered in `config.yaml` as `harnez-init` pointing to `docs/commands/HarnezInit.md` with `preset_skill_overrides` configured as `user-invocable-only`.

## 4. Verification

- **Uninitialized Repo Test**: Tested on clean git repository (`/tmp/harnez-test-uninit-*`). Guard correctly halts execution without running `harnez init` and prompts the user for setup options.
- **Initialized Repo Test**: Tested on initialized git repository (`/tmp/harnez-test-init-*`). Re-run executed cleanly, diff assessed for broken links and regressions without errors.
- **Automated Tests**: Unit test `internal/claude/harnez_init_skill_test.go` and full test suite (`make test`) pass cleanly.

/goal Ship the `/harnez-init` skill with the guard and assess steps, verified by
running it in one initialized and one uninitialized repo; stop and report if blocked
on a user decision or denied permission.
