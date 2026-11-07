# 1184 — harnez release --continue fails if annotated git tag is missing

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #826, #863

---

## 1. Problem & Motivation
When running `harnez release --continue` to resume an interrupted release, the command assumes that the annotated Git tag for the target version already exists locally. If the release was interrupted before tagging succeeded, or if the local tag was removed during troubleshooting/recovery, `harnez release --continue` proceeds straight to `pushRelease` without checking for the tag, causing git push preflight to fail with:

```text
Error: release push preflight failed: remote <remote>: exit status 1: error: src refspec refs/tags/<tag> does not match any
```

## 2. Technical Specification / Findings
In `internal/release/runner.go`:
- Step 4 (`Version determination & spec handling`) only invokes `gitCreateTag(opt.Dir, tagName, tagMsg)` in the `!opt.Continue` branch.
- In the `opt.Continue` branch, `tagName` is computed via `FormatTag(tagPrefix, targetVersion)`, but `tagExists(opt.Dir, tagName)` is never checked or ensured.
- Step 8 (`Remote Git Push`) passes `tagName` to `pushRelease()`, which attempts to push `refs/tags/<tagName>:refs/tags/<tagName>` to all configured remotes.

If `opt.Continue` is active and `tagExists(opt.Dir, tagName)` returns false:
- Verify that `HEAD` contains the release version commit (or matches `targetVersion`).
- Recreate the annotated tag `gitCreateTag(opt.Dir, tagName, fmt.Sprintf("%s %s", projectName, tagName))` before proceeding to the push step.

## 3. Implementation & Verification Plan
1. In `internal/release/runner.go` (under `if opt.Continue` or right before Step 8 push):
   - Check if `tagExists(opt.Dir, tagName)` is true.
   - If false, log `[git] Recreating missing tag: <tag>` and call `gitCreateTag(opt.Dir, tagName, tagMsg)`.
2. Unit tests in `internal/release/resume_test.go`:
   - Simulate resuming a release where dist/commit exists but the git tag is missing.
   - Verify `--continue` recreates the tag and succeeds without `src refspec refs/tags/... does not match any`.
