# 308 — Validate go.mod module name against git remote origin and directory name in init

**Status**: Closed — implemented go.mod vs git origin and directory name drift validation in init
**Priority**: P1 (High)
**Severity**: Major
**Category**: Reliability
**Related**: `internal/claude/init.go`, `cmd/harnez/main.go`

---

## 1. Problem & Motivation

When initializing or synchronizing project scaffolding with `harnez init`, if a project's `go.mod` declares a module whose base name does not match the Git remote repository origin or the project directory name (e.g. `go.mod` is `ubunatic.com/uman` in a directory named `psync` whose origin is `ubunatic/psync.git`), this signals severe repository drift or an accidental clone/copy.

Running `harnez init` without detecting this can cause automated tools and agents to generate invalid conventions, docs, and workspace entries without warning the user.

## 2. Proposed Solution

1. In `RunInit` / `init.go`:
   - Inspect `go.mod` (if present) for the module name.
   - Inspect `git remote get-url origin` (if inside a Git repository).
   - If the module path basename does not match the Git remote repository name or the project directory basename, emit a clear warning indicating potential repository drift.
2. Provide informative diagnostic output so the user immediately knows if `go.mod`, git remote, or folder name are mismatched.

## 3. Acceptance Criteria

- [ ] `harnez init` checks for module name vs git remote repository name / directory name consistency.
- [ ] Warning is printed when a mismatch is detected.
- [ ] Unit tests cover mismatch detection.
