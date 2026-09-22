# 503 — Release publishes stale versioned source archive

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: `harnez release`

## 1. Problem & Motivation

Running `harnez release` in `../loom` for tag `v0.2.5` reported success after uploading
`dist/loom-v0.2.2.tar.gz` alongside the v0.2.5 release. This can leave users downloading
source artifacts that do not match the published release.

## 2. Technical Specification / Findings

The cause is not yet known. Determine why the release workflow selected the older
versioned archive and whether other generated assets can also be stale.

## 3. Implementation & Verification Plan

**Goal**: Ensure `harnez release` publishes versioned artifacts that match the tag being
released, and verify the resulting assets for a release build.
