# 745 — skillreg checkout fails on git 2.43: --end-of-options read as pathspec

**Status**: Closed — resolved
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: #743

---

## 1. Problem & Motivation
Installing or updating a pinned skill fails on git 2.43 (Ubuntu 24.04's default):
`error: pathspec '--end-of-options' did not match any file(s) known to git`.
This hits users, not only tests (`TestPinnedRefInstall`,
`TestUpdatePinnedRefStaysUnlessLatest`, `TestUpdateWithPrunedCacheAndRename`).

## 2. Technical Specification / Findings
`internal/skillreg/skillreg.go:724`: `git checkout --quiet --end-of-options <ref>`.
git 2.43 `checkout` treats `--end-of-options` as a pathspec.

## 3. Implementation & Verification Plan
- Reject refs starting with `-` before calling git, then use
  `git checkout --quiet --detach <ref> --` (works on all supported git versions).
- Verify: the three tests pass on git 2.43 and on current git.

## 4. Resolution
- Replaced `--end-of-options` with `--detach <ref> --` in `internal/skillreg/skillreg.go`.
- Added container test setup (`scripts/test-ubuntu.sh` and `make test-ubuntu`) to verify tests against Ubuntu 24.04 (git 2.43) using Podman or Docker.
- `scripts/install-canary.sh` auto-detects `CONTAINER_CMD` (podman or docker);
  `scripts/agent-canary/run.sh` stays podman-only (pods).
- Verified on git 2.43.0: `go test ./internal/skillreg/` passes. Full
  `make test-ubuntu` still fails on unrelated tests, see #749.
