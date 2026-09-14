# 338 — macOS CI verification via GitHub mirror and Homebrew release packaging

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Build & Distribution (macOS Delivery)
**Category**: Packaging & CI / Multi-OS
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md)

---

## 1. Problem & Motivation

`harnez` builds native macOS binaries (`harnez_darwin_amd64` and `harnez_darwin_arm64`) via `.goreleaser.yaml`, but currently has no continuous integration (CI) pipeline running on macOS hosts to verify build health, runtime execution, and tests against real Darwin kernels.

Codeberg (the primary git forge) does not provide native macOS runner instances. However, the repository has a synced GitHub mirror (`git@github.com:ubunatic/harnez.git`), which supports GitHub Actions with native macOS runners (`macos-latest` / Apple Silicon).

Additionally, distributing `harnez` on macOS is best served via a Homebrew tap (`brew install ubunatic/tap/harnez`) in addition to raw tarball archives.

## 2. Technical Specification

1. **GitHub Actions Workflow for macOS CI**:
   - Create `.github/workflows/macos-ci.yaml` (triggered on push/PR mirrored to GitHub).
   - Run matrix builds and tests on macOS (`macos-latest` / `macos-14` Apple Silicon).
   - Run `go test ./...` and `harnez` CLI smoke tests on Darwin.
2. **GoReleaser Homebrew Tap Integration**:
   - Configure GoReleaser v2 `brews` block in `.goreleaser.yaml` to publish formula definitions to a Homebrew tap repository upon release.
   - Verify non-interactive binary packaging, man page installation, and completions for Homebrew users.

## 3. Implementation & Verification Plan

1. Add macOS workflow `.github/workflows/macos-ci.yaml`.
2. Configure Homebrew formula generation in `.goreleaser.yaml`.
3. Test workflow execution against the GitHub mirror.
4. Verify end-to-end `brew install` flow on a real or virtual macOS machine.
