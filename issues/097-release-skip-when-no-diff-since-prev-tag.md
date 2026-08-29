# 097 — `harnez release` should skip when there's no diff since the previous tag

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `internal/release/runner.go`, `cmd/harnez/release.go`, [[091-language-agnostic-release-spec-and-thin-make-release]]

## Problem

`Run` (`internal/release/runner.go:28`) has no check for whether anything
actually changed since the last release tag. Preflight only verifies the
toolchain, the minisign key, and that the working tree is clean
(`isGitClean`, line ~69) — it never compares HEAD against the previous tag.
So a bump-and-tag-and-publish cycle runs unconditionally, even when the
previous tag already points at (or is equivalent to) the current code —
producing a version bump and a published release with an empty diff.

## Proposed Fix

Before the version-bump/tag step (~line 79), resolve the previous release
tag (existing tag-naming logic already in this function via `bumped.TagName()`
gives the shape; use `git describe --tags --abbrev=0` or an equivalent git
call to find it) and check for a real diff between it and HEAD, e.g.
`git diff --quiet <prev-tag>..HEAD` or `git rev-list <prev-tag>..HEAD --count`.

- If there's no diff: print a message (e.g. `No changes since <prev-tag> —
  nothing to release. Use --force to re-release the current code as a new
  version.`) and exit 0 without bumping, tagging, or publishing.
- Add a `--force` flag (`cmd/harnez/release.go`, alongside the existing
  `--continue`/`--dry-run`/`--skip-*` flags) to explicitly bypass this check
  and re-release the current code under a new version anyway.
- `--continue` should probably imply skipping this check too, since it's
  already resuming a specific in-progress release rather than starting a
  fresh one — needs confirming against `--continue`'s existing semantics
  (lines 82–93) before implementing.
- No first release exists yet (no previous tag): the check has nothing to
  compare against, so it should be a no-op — proceed as today.

## Notes

Where exactly to source "the previous tag" needs one decision: the most
recent tag reachable from HEAD (`git describe --tags --abbrev=0`), vs. the
tag matching the currently recorded version (`AutoDetectCurrentVersion` /
`VersionSpec.Version`, already used a few lines below at ~line 100). These
should normally agree, but could diverge if a tag was created outside
`harnez release` — prefer whichever this function already treats as the
source of truth for "current version" to avoid a second, possibly
inconsistent notion of "current".
