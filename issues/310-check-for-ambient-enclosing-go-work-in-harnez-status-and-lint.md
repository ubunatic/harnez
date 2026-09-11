# 310 — Check for ambient enclosing go.work in harnez status and lint

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Infrastructure
**Related**: [issues/287](287-go-work-ambient-workspace-file-breaks-unrelated-tooling-in-sibling-repos-general-fix-needed.md), [issues/309](309-manage-go-workspace-via-go-work-example-and-symlink-with-init-gowork.md), [internal/lint](../internal/lint), [cmd/harnez/main.go](../cmd/harnez/main.go)

---

## 1. Problem & Motivation

When a Go workspace file (`go.work`) exists in an enclosing parent directory (such as `~/projects/go.work`), Go's automatic upward directory walk silently captures all sibling projects and child commands. This causes unexpected build errors, tool failures (`directory ... is not one of the workspace modules`), and type pollution across unrelated projects.

While `harnez init --gowork` (issue 309) establishes project-local `go.work.example` and untracked symlinks, developers or agents might inadvertently create or leave a `go.work` in a parent directory.

`harnez status` and `harnez lint` should proactively detect and warn about any enclosing parent `go.work` that is not part of the current project root, alerting the developer before silent collateral breakage occurs.

## 2. Technical Specification

1. **Ambient Workspace Detection Helper**:
   - Query `go env GOWORK` (or walk upwards from `dir` to `$HOME` / `/`).
   - If a `go.work` is found in a parent directory above the project root:
     - Check whether `GOWORK` environment variable is explicitly set (e.g. `GOWORK=off` or an explicit file path).
     - If ambient parent `go.work` is active, identify the parent workspace path.
2. **`harnez status` Audit**:
   - Under workspace diagnostics, report:
     - `⚠️  ambient go.work detected at <parent-path> (may cause cross-repo module leakage; remove or use project-local go.work)`
3. **`harnez lint` Check**:
   - Add a lint rule (e.g. `ambient-gowork`) that reports a warning/drift when an ambient parent workspace is active above a repository that has Go modules.

## 3. Implementation & Verification Plan

- Add ambient workspace detection in `internal/lint` or `internal/claude`.
- Integrate diagnostic reporting into `harnez status` and `harnez lint`.
- Add unit tests verifying warning output when a parent directory contains a `go.work` file.
