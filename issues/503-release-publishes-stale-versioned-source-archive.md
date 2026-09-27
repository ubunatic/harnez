# 503 — Release publishes stale versioned source archive

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: `harnez release`

## 1. Problem & Motivation

Running `harnez release` in `../loom` for tag `v0.2.5` reported success after uploading
`dist/loom-v0.2.2.tar.gz` alongside the v0.2.5 release. This can leave users downloading
source artifacts that do not match the published release.

## 2. Technical Specification / Findings

1. `runBuildStep` skips artifact builds when no `.goreleaser.yaml`, `--build-cmd`, or `make pack`/`dist` target is found, but leaves existing `dist/` directory contents intact.
2. `runSigningStep` blindly signs whatever `dist/SHA256SUMS` exists, even if it was created by an earlier release.
3. `runPublishStep` attaches all archives and project-prefixed assets from `dist/` without verifying that versioned filenames match `targetVersion` / `tagName`.
4. Plain library projects without binaries/archives should clean/isolate `dist/` or omit uploading unmatched artifacts, while projects with binaries (CLI/TUI) must build fresh matching artifacts.

## 3. Implementation & Verification Plan

- **M1 (reproduction test)**: Added tests in `internal/release/release_test.go` confirming stale archive selection and skipped build artifact leak. Delivered in commit `1a0620a`.
- **M2 (artifact isolation and version filtering)**:
  - **Pre-Work / Requirements**:
    1. Clean or reset `dist/` before running the build step (or when starting a fresh release), so stale archives from prior versions never linger.
    2. In `runPublishStep` and `runSigningStep`, validate that versioned filenames (e.g. `*v0.2.2*` vs `v0.2.8`) matching `<project>-*` or containing semver/tags match the current `targetVersion` / `tagName`, or reject them if mismatched.
    3. If artifact build is skipped and no fresh artifacts are built for the current version, do not publish orphaned/stale assets or checksums.
    4. Ensure both plain libraries (no binary artifacts) and CLI/TUI projects (with GoReleaser or custom build cmds) are handled cleanly without false-positive failures or stale asset leaks.
    5. Update tests to assert that stale artifacts are ignored/cleaned and only current-version artifacts are signed and published.
