---
name: harnez-init
description: Re-run harnez init in an initialized repository, assess the diff for regressions, fix local fallout, and commit changes
disable-model-invocation: true
---

# Harnez Init

Re-sync an initialized repository, assess regressions, fix local fallout and commit.

### 1. Guard: Verify Prior Initialization

- Before modifying files or running init, check for `.harnez/` at the repository root and
  `<!-- harnez:begin` markers in `AGENTS.md` or `CLAUDE.md`.
- If NOT initialized, Stop immediately; request explicit choices and flags for Doc variant
  (`--variant lite` or `--variant full`), Quota guardrails, Documentation set and Repository mode
  (solo, fork or team). Wait for confirmation.

### 2. Re-run Init

- Run `harnez init`, passing any flags supplied by the user.

### 3. Assess the Diff

- Inspect modified and untracked files with `git diff` and `git status`; check for Broken links,
  Self-referential pointers, Contradicting rules and Lost local edits (instructions, Makefile
  targets or notes).

### 4. Fix Local Fallout

- Fix small local fallout only where the fix survives another init. Never hand-edit managed
  `harnez:begin` / `harnez:end` blocks or bundled doc copies that init overwrites.

### 5. File Upstream Issues

- For defects in Harnez templates, bundled sources or CLI generation, file an upstream issue
  with the consumer repository, path, line numbers and diff evidence; if Harnez is inaccessible,
  report the details to the user.

### 6. Commit and Report

- Check status, stage and commit clean init changes with a conventional commit; push only on
  explicit request.
- Report each changed or added file, regression findings, local fixes, upstream issues, commit
  SHA and final working tree status.
