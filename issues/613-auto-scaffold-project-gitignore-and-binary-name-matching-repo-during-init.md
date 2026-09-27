# 613 — Auto-scaffold project gitignore and binary name matching repo during init

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation

When initializing a new Go repository with `harnez init`, the generated `Makefile` defaults to a placeholder `BINARY := myapp`, and `.gitignore` is not automatically scaffolded. This leads to accidental untracked binaries or manual fixes required immediately after init.

According to `docs/Go.md`:
> "Use the BINARY variable from the Makefile as the canonical name so the .gitignore entry and the build output always match."

## 2. Technical Specification / Findings

- `harnez init` should detect the directory basename (e.g. `loom-games` or module name from `go.mod`).
- Set `BINARY := <name>` in the generated `Makefile`.
- Create or append to `.gitignore`:
  ```
  # ignore Go binaries
  /<name>
  ```

## 3. Implementation & Verification Plan

### Goal
Ensure new project initialization scaffolds `.gitignore` and `Makefile` with the repository's canonical binary name.

### Acceptance Criteria
- [ ] `harnez init` populates `Makefile` with the repository name instead of `myapp`.
- [ ] `harnez init` ensures `/<binary>` is ignored in `.gitignore`.
- [ ] Unit tests for `harnez init` templates.
