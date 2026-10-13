# 744 — release tests require minisign on PATH

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug / Tests
**Related**: #743

---

## 1. Problem & Motivation
`TestDryRunRelease`, `TestDryRunReleaseContinue`, `TestPreflightForgeValidation`,
`TestTagPrefixFormatting` and `TestBuildCmdMakefileFallback` fail with
`required tool "minisign" not found in PATH` on hosts without minisign.

## 2. Technical Specification / Findings
`internal/release/runner.go:278` adds `minisign` to the required tools; the preflight
runs before dry-run short-circuits signing. `internal/release/minisign.go:71` shells out.

## 3. Implementation & Verification Plan
- Tests put a fake `minisign` script in a `t.TempDir()` on PATH (as other tool fakes do),
  or dry-run checks only tools it actually runs.
- Verify: tests pass with `minisign` absent from PATH; a real release still requires it.
