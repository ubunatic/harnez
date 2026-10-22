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

## Way forward

- Opt-in source of truth in `.harnez/` (e.g. `.harnez/config.yaml`: `issues: harnez`, `studies: harnez`),
  written once by init when it detects the layout (854) or by the wizard (855), and respected afterwards.
- The existing `managed by harnez index` header (847) can serve as the per-file marker: never rewrite an index
  file that lacks it unless the repo config opts in.
- `--issues-git` stays as an explicit override and updates the repo config.

## Checks

- Scratch repo with a foreign `issues/README.md` (no marker, no config): init changes nothing there and
  installs no driver.
- This repo: behavior unchanged, `git status` clean after init.
