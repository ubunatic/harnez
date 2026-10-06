# 734 — harnez release publishes without artifacts when there is no dist dir

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [#091 language-agnostic release](091-language-agnostic-release-spec-and-thin-make-release.md)

---

## 1. Problem & Motivation
`harnez release` in a repo without a build setup (no `.goreleaser.yaml`, no
`--build-cmd`, no Makefile build target) already skips the build with a
notice, then fails in the publish step because `dist/` is missing or empty.
Repos without binaries (docs, scripts, skills) cannot be released.

/goal When there is no `dist/` or it holds no release artifacts, `harnez
release` prints a warning and publishes the forge release without
attachments; tag, commit and push work as before. Stop and report if `fj`
cannot create a release without attachments.

## 2. Technical Specification / Findings
- Fail points in `internal/release/runner.go` `runPublishStep`: missing
  `dist/` ("dist directory ... does not exist; cannot publish") and no
  matching files ("no release artifacts found in dist/").
- Dry-run already takes the no-artifact path for both cases.
- Signing already skips without `dist/SHA256SUMS`.
- `PublishForgejoRelease` (`forge.go`) loops over attachments and has a
  bare-release fallback; check it works with an empty list.

## 3. Implementation & Verification Plan
- Test: release run against a temp repo without `dist/` warns and calls
  publish with zero attachments.
- Live check on a repo without a goreleaser config.
