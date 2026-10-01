---
name: harnez-init
description: Re-run harnez init in an initialized repository, assess the diff for regressions, fix local fallout, and commit changes
disable-model-invocation: true
---

# Harnez Init

Re-sync an already initialized consumer repository with the latest harnez conventions, assess the diff for regressions, resolve local fallout, file upstream issues for harnez bugs, and commit the changes.

## Workflow

### 1. Guard: Verify Prior Initialization
Before modifying any files or running commands, check whether the current repository was already initialized by Harnez:
- Look for `.harnez/` directory in the repository root.
- Look for `<!-- harnez:begin` markers in `AGENTS.md` or `CLAUDE.md`.

**If NOT initialized**:
- **Stop immediately** without running `harnez init` or editing files.
- Explain to the user that the repository has not yet been initialized.
- Prompt the user to choose their preferred initial setup options:
  1. **Doc variant**: Lite vs Full docs (`--variant lite` vs `--variant full`).
  2. **Quota guardrails**: With or without Quota-1 (`--quota-1`).
  3. **Documentation set**: Which language or practice docs to add (e.g. `golang`, `bash`, `agentic-loop`, `issue-tracking`, `spec`, `canary` via `--docs <names>`).
  4. **Repository mode**: Solo (`-m solo`), Fork (`-m fork`), or Team (`-m team`).
- **Rule**: First-time repository initialization is a user decision, never an agent assumption. Wait for user confirmation and explicit flags.

### 2. Re-run Init
When the repository is already initialized:
- Run `harnez init` (or `harnez init -d .`). If specific flags were supplied by the user, pass them along.

### 3. Assess the Diff
Inspect all modified and untracked files with `git diff` and `git status`:
- Provide a concise, one-line explanation for each changed or added file.
- Actively scan the diff for regressions and defects:
  - **Broken links**: Check if relative links (`@docs/...`, `See docs/...`, markdown links) in updated docs point to files that do not exist in the consumer repository.
  - **Self-referential pointers**: Check if doc headers or notes point to the file itself instead of a valid source or upstream URL (e.g. issue 667).
  - **Contradicting rules**: Check if newly generated rules conflict with unmanaged repo-specific policies outside `harnez:begin` blocks.
  - **Lost local edits**: Verify that custom instructions, Makefile targets, or notes outside managed blocks were not accidentally overwritten or lost.

### 4. Fix Local Fallout
- Address small local fallout that survives future `harnez init` runs (e.g. local sections in `AGENTS.md` outside managed blocks, local Makefile targets, repository-specific links).
- **Rule**: Never hand-edit content inside `harnez:begin` / `harnez:end` markers or bundled doc copies if `harnez init` would overwrite those changes on the next run.

### 5. File Upstream Issues
- When a defect or regression originates from Harnez templates, bundled doc sources, or CLI generation logic:
  - Do not apply fragile local hacks inside generated files.
  - File an issue in the Harnez repository with concrete evidence (consumer repository name, file path, line numbers, and diff excerpt). If working outside the Harnez repo, report the bug details clearly to the user or file it when Harnez is accessible.

### 6. Commit and Report
- In the consumer repo, check `git status`.
- Stage and commit the clean init changes with a conventional commit message (e.g. `chore: re-sync harnez init configuration and docs`).
- Do **not** push to remotes unless explicitly requested.
- Report a summary containing:
  - Updated/added files with one-line descriptions.
  - Regression assessment findings.
  - Local fixes made (if any).
  - Upstream issues filed (if any).
  - Commit SHA and final working tree status.
