# 769 — release: detectGoPackage emits invalid Go syntax when directory name is a reserved keyword

**Status**: Closed — Validated fallback package names with go/token; keyword and invalid directory names now use package main, while gotool remains unchanged. Regression coverage is in TestDetectGoPackageFallbackValidatesDirectoryName; go test ./internal/release/... passes.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #091, #734

---

## 1. Problem & Motivation

When `harnez release` runs on a Go project that has no `files:` defined in `version.yaml` and no root `version.go`, it attempts to automatically generate a `version.go` file at the repository root.

To determine the package name for the new file, `detectGoPackage` in `internal/release/sync.go` falls back to the repository directory's basename (`filepath.Base(dir)`). If the repository or directory name matches a reserved Go keyword (such as `goto`, `select`, `default`, `type`, `map`, etc.), it generates uncompilable Go code:
```go
package goto

var Version = "0.1.1"
```
This causes any subsequent `go vet` or `go build` to fail with `expected 'IDENT', found '<keyword>'`.

## 2. Technical Specification / Findings

- Location: `internal/release/sync.go` in function `detectGoPackage(dir string) string`.
- When falling back to `base := filepath.Base(dir)`:
  - If `token.Lookup(base).IsKeyword()` (using Go's `go/token` package) is true, or if `base` is not a valid Go identifier:
    - Default to `package main` instead of the reserved keyword name, or emit an actionable error/warning directing the user to define `files:` in `version.yaml`.
- Also consider searching `cmd/*/*.go` for existing `var Version` declarations before creating a root `version.go`, so existing layouts like `cmd/<app>/version.go` are discovered automatically.

## 3. Implementation & Verification Plan

- Add a unit test in `internal/release/release_test.go` verifying that running version sync on a directory named `goto` (or another keyword) generates `package main` or successfully detects a child `cmd/<name>/version.go`, without generating syntax errors.
- Verify `make test` passes in `harnez`.
