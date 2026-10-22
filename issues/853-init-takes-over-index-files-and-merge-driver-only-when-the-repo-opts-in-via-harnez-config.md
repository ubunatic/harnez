# 853 — init takes over index files and merge driver only when the repo opts in via .harnez config

**Status**: Open
**Priority**: P2 (Medium) — init can claim files in repos that use a different tracker
**Severity**: Major
**Category**: Bug
**Related**: 847 (merge driver on by default), 848, 854 (generic init)

---

/goal `harnez init` writes the generated index files (issues/README.md, BACKLOG.md, CLOSED.md, docs/README.md
studies index) and installs the `harnez-issues-index` merge driver and hook only in repos that use the harnez
issues/ and docs/studies layout, as recorded in per-repo config under `.harnez/` (or an index file's
frontmatter marker); other repos are left untouched. Verified in scratch repos, or stop and report when blocked
on a design decision.

## Findings (2026-10-22)

- `internal/claude/init_agent_file.go:97`: integration is installed when `--issues-git` is unset and an
  `issues/` dir exists. Any `issues/` folder qualifies, even a different tracker format.
- `internal/claude/operation_options.go:14` says "nil preserves the integration"; the code installs it.
  Comment and behavior disagree.
- Research pass (2026-10-22): the installed rules use the same weak gate. `.harnez/rules/Issues.md:3-9` and
  `.harnez/rules/Tools.md:39,51-55` apply harnez tracker conventions to any `issues/` dir. Gate them on the
  same opt-in, else defer to the repo's own tracker rules.
- The post-merge hook runs `harnez index`, which errors when neither `issues/` nor `docs/README.md` exists
  (`cmd/harnez/index.go:100`).
- On fix, update `docs/CLIDesign.md` init flow (marked "to become opt-in (853)") and README's issue-tracking
  intro.

## Way forward

- Opt-in source of truth in `.harnez/` (e.g. `.harnez/config.yaml`: `issues: harnez`, `studies: harnez`),
  written once by init when it detects the layout (854) or by the wizard (855), and respected afterwards.
- The existing `managed by harnez index` header (847) can serve as the per-file marker: never rewrite an index
  file that lacks it unless the repo config opts in.
- `--issues-git` stays as an explicit override and updates the repo config.

## Sprint 2026-10-22 (after 854)

854 delivered `.harnez/repo.yaml` (`issues.tracker`, `issues.merge_driver`) and the agent notice in
`.harnez/rules/Index.md`. This ticket now means: init and `harnez index` read those keys instead of "any
`issues/` dir"; `--issues-git` updates `issues.merge_driver`; a repo with `tracker: none` gets no
attributes, driver, hook or index rewrite, and an existing harnez-installed integration is removed when the
config turns it off. Fix the `operation_options.go:14` comment. `harnez index` in a repo with neither
`issues/` nor `docs/README.md` exits 0 with a short note instead of failing (hook safety).

Host repro (854 M2): scratch repo with a plain Makefile and a foreign `issues/README.md` → repo.yaml says
`merge_driver: false`, yet `.gitattributes` and `merge.harnez-issues-index.*` were installed.

## Checks

- Scratch repo with a foreign `issues/README.md` (no marker): init records `tracker: none`; no
  `.gitattributes`, merge config, or active hook is installed. `cmp` confirms the README is unchanged
  after init and `harnez index`.
- This repo: `make install` and `harnez init -d .` report no changes.
- `make test-q1`: passed; full output saved to `/tmp/harnez-853-test-q1-final.log`; `rg -- '--- FAIL'`
  found no matches.

## Implementation (2026-10-22)

- Init installs issue-index Git integration only when `issues.tracker: harnez` and
  `issues.merge_driver: true`; disabled config removes only Harnez-managed attributes, merge settings,
  and hook blocks. `--issues-git` choices persist in `repo.yaml`.
- `harnez index` gates issues and studies indexes independently and prints a successful no-op note when
  neither enabled index is present.
- The file-first tracker remains the recommended default. This ticket remains open for the host to close.
