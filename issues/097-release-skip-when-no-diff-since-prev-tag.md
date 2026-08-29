# 097 — `harnez release` should skip when there's no diff since the previous tag

**Status**: Closed — resolved
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

## Progress (2026-08-29)

Implemented in `internal/release/runner.go`:

- Added `Options.Force` field and a `--force` flag in `cmd/harnez/release.go`
  (no name collision with existing flags).
- In the non-`--continue` branch of `Run`, right before `BumpVersion` is
  called, resolve `prevTag := "v" + strings.TrimPrefix(currentVersion, "v")`
  — i.e. the tag matching the currently recorded version (`spec.Version` or
  `AutoDetectCurrentVersion`), the same source of truth already used for
  version determination a few lines below, per the Notes above.
- New helpers `tagExists(dir, tag)` and `hasDiffSinceTag(dir, tag)`
  (`git rev-parse --verify --quiet refs/tags/<tag>` and
  `git diff --quiet <tag> HEAD` respectively). Any failure to resolve the tag
  (missing tag, or `dir` not being a git repo at all — a brand-new project
  with no prior release) is treated as "no previous tag", so first-ever
  releases proceed unimpeded — this also covers the "no first release yet"
  edge case without a special-cased branch.
- If the tag exists and there's no diff to `HEAD`, and `--force` was not
  passed: print `No changes since <prev-tag> — nothing to release. Use
  --force to re-release the current code as a new version.` and return `nil`
  (exit 0) before any bump/commit/tag/build/publish step runs.
- `--continue` skips this check entirely (it lives only in the fresh-release
  branch), matching the ticket's proposed semantics — `--continue` resumes a
  specific in-progress release rather than starting a fresh one.
- The check runs even under `--dry-run` (harmless — read-only git calls) so
  a dry-run preview also reports "nothing to release" accurately.

Tests added to `internal/release/release_test.go`:

- `TestTagExistsAndHasDiffSinceTag` — direct unit coverage of the two new
  helpers against a real temp git repo fixture (tag exists / doesn't exist;
  no diff before a new commit / diff present after one).
- `TestReleaseSkipWhenNoDiffSincePrevTag` — table test driving `Run()`
  end-to-end via a temp git repo fixture (`setupGitRepoWithVersion` /
  `runGitCmd` helpers, new — no existing repo-with-commits fixture existed
  in this file), covering all four required cases:
  - `no diff skips release`
  - `diff present proceeds`
  - `force flag overrides no-diff skip`
  - `no prior tag proceeds`

Verified: `go build ./...`, `go vet ./...`, `make check` (`go test ./...`,
full suite including the new tests), and `make install` — all pass.
