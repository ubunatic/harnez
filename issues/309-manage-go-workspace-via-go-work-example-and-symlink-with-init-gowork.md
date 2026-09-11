# 309 — Manage Go workspace via go.work.example and symlink with init --gowork

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Infrastructure
**Related**: [issues/287](287-go-work-ambient-workspace-file-breaks-unrelated-tooling-in-sibling-repos-general-fix-needed.md), [internal/claude/gowork.go](../internal/claude/gowork.go), [cmd/harnez/main.go](../cmd/harnez/main.go)

---

## 1. Problem & Motivation

Parent/ambient `go.work` files cause unexpected workspace errors when running sub-modules or tools in sibling repositories. To prevent this, repositories should be self-contained with their own local workspace configurations when needed.

However, committing a `go.work` file with relative paths (e.g. `../loom`, `../voxi`) breaks external checkouts, `go install`, Docker builds, and CI runners.

The solution is the `go.work.example` pattern:
1. `go.work.example` is committed to git with a header comment `// harnez:managed`.
2. A local symlink `go.work -> go.work.example` is created for development.
3. `.gitignore` ignores `/go.work` and `/go.work.sum`.

## 2. Technical Specification

1. **Flag `--gowork` in `harnez init`**:
   - Add `--gowork` flag to `harnez init`.
2. **Behavior Matrix**:
   - If no `go.work` or `go.work.example` exists:
     - By default (`init` without `--gowork`): skip workspace creation (do not create unnecessary files).
     - With `init --gowork`: generate `go.work.example` with `// harnez:managed`, create symlink `go.work -> go.work.example`, and ensure `.gitignore` ignores `/go.work` and `/go.work.sum`.
   - If `go.work.example` or `go.work` has `// harnez:managed` or `go.work` is a symlink:
     - Always maintain symlink and ensure `.gitignore`.
   - If `go.work` is tracked in git (without `// harnez:managed`):
     - By default: skip with notice: `go.work is tracked in git; skipping workspace management (use --gowork or add '// harnez:managed' to opt in)`.
     - With `--gowork`: migrate tracked `go.work` to `go.work.example`, untrack `go.work`, create symlink, and update `.gitignore`.

## 3. Implementation & Verification Plan

- Update `internal/claude/gowork.go` to support `go.work.example`, symlinks, `.gitignore` reconciliation, and `// harnez:managed` marker detection.
- Add `--gowork` flag to Cobra CLI in `cmd/harnez/main.go` and thread to `init.go`.
- Add comprehensive unit tests in `internal/claude/gowork_test.go`.
- Verify with `go test ./...` in `harnez` and `make install`.
