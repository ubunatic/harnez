---
name: harnez-init
description: Re-run harnez init in an initialized repository, assess the diff for regressions, fix local fallout, and commit changes
disable-model-invocation: true
---

# Harnez Init

Re-sync an initialized repository, assess regressions, fix local fallout and commit.

- Before modifying files or running init, check for `.harnez/` at the repository root and
  `<!-- harnez:begin` markers in `AGENTS.md` or `CLAUDE.md`.
- If not initialized, stop; explain that first-time setup needs the user's explicit choices and
  flags for doc variant (lite or full), Quota-1, documentation set and repository mode (solo,
  fork or team). Wait for confirmation.
- Otherwise run `harnez init`, passing any flags supplied by the user.
- Inspect modified and untracked files with `git diff` and `git status`; check for broken relative
  links, self-referential pointers, conflicts with unmanaged rules and lost local instructions,
  Makefile targets or notes.
- Fix small local fallout only where the fix survives another init. Never hand-edit managed
  `harnez:begin` / `harnez:end` blocks or bundled doc copies that init overwrites.
- For defects in Harnez templates, bundled sources or CLI generation, file an upstream issue
  with the consumer repository, path, line numbers and diff evidence; if Harnez is inaccessible,
  report the details to the user.
- Check status, stage and commit clean init changes with a conventional commit; push only on
  explicit request.
- Report each changed or added file, regression findings, local fixes, upstream issues, commit
  SHA and final working tree status.
