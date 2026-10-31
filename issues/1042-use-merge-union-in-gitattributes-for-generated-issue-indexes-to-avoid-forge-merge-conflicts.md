# 1042 — Use merge=union in .gitattributes for generated issue indexes to avoid forge merge conflicts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: [[282]], [[847]], [[853]], [[1040]], [[1041]], [[1045]], [[1055]], docs/IssueTracking.md

---

## 1. Problem & Motivation
When concurrent topic branches create or update tickets, each branch modifies `issues/README.md`.
Locally, the custom merge driver resolves merges cleanly. Remote Git hosting platforms (Forgejo /
Codeberg, GitHub) cannot execute clone-local custom merge drivers from `.git/config`, so they may
flag spurious conflicts and mark PRs unmergeable even when each PR touches disjoint issue files.

The original proposal was the built-in `merge=union` attribute, on the assumption that forges honor it.

## 2. Current State (refined 2026-10-31)

Commit `2a7c9bee` already implemented the proposal and is on `origin/main`, but the ticket stayed Open.
The assumption it rests on was never verified, and the evidence below points against it. The remaining
work is to verify it live, then keep or revert the change.

### Already in place
- `issuesAttributesLines()` (`internal/claude/init.go:1324`) writes `merge=union` for all four paths:
  `issues/README.md`, `issues/BACKLOG.md`, `issues/CLOSED.md`, `docs/README.md`.
- `installIssuesGitIntegration` (`internal/claude/init.go:1372`) rewrites legacy
  `merge=harnez-issues-index` lines and sets both `merge.union.driver` and
  `merge.harnez-issues-index.driver` to `harnez issues merge-driver %O %A %B %P`.
- `runIssuesRebase` (`internal/issues/maintenance/rebase.go:174`) accepts both attribute values.
  It requires a configured driver only for the legacy value.
- This repository's `.gitattributes` and `docs/practices/IssueTracking{,.lite}.md` already say `merge=union`.

### Finding 1: Forgejo's conflict check probably ignores `merge=union` (unproven on Codeberg)
- Forgejo's `testPatch` runs `git merge-tree --write-tree` in the bare base repository
  (`services/pull/patch.go`, `forgejo` branch, read 2026-10-31). It passes no `--attr-source`, and its
  built-in config in `modules/git/git.go` does not set `attr.tree`. Its pre-2.38 fallback uses
  `git merge-file`, which does not read attributes either.
- Local canary, git 2.56, two branches adding different rows at the same spot:

  | Command | Result |
  |---------|--------|
  | non-bare `git merge-tree --write-tree` | exit 0, unioned |
  | bare `git merge-tree --write-tree`, `.gitattributes` committed on HEAD | exit 1, CONFLICT |
  | bare with `--attr-source=main` | exit 0 |
  | bare with `-c attr.tree=main` | exit 0 |
  | bare with `info/attributes` | exit 0 |

- Not ruled out: `modules/git/git.go:209` applies admin-set `[git.config]` values, so Codeberg could
  set `attr.tree`. GitHub's behavior is unknown. Only milestone M1 settles both.
- `docs/practices/IssueTracking.md:196-198` currently states the forge benefit as fact.

### Finding 2: `merge.union.driver` breaks unrelated `merge=union` paths (reproduced)
A local `merge.union.driver` overrides Git's built-in `union`. In any repository after `harnez init`,
a user's own `CHANGELOG.md merge=union` line now routes to the harnez driver, which fails with
`issues merge-driver: unsupported generated path "CHANGELOG.md"` and turns a clean union into a
conflict. `removeIssuesGitIntegration` also unsets `merge.union.*` without checking that harnez set them.

### Finding 3: the built-in union is weaker than the driver (reproduced)
A clone with the committed attributes but without `merge.union.driver` uses the built-in union. This
checkout is in that state: only `merge.harnez-issues-index.*` is configured. The built-in union:
- keeps a row that one side deleted when the other side edited that row, until `harnez index` runs;
- silently keeps both sides of competing hand-written lines in `docs/README.md`, where the driver refuses.

### Backward compatibility

| Repository state | Behavior today | Test coverage |
|------------------|----------------|---------------|
| Legacy attributes and legacy driver, init not rerun | Driver runs; rebase accepted | `rebase_test.go` |
| Legacy attributes, no driver | Rebase refuses and names `harnez init --issues-git` | `TestIssuesRebaseMissingIntegrationShowsExactSetupCommand` covers unset attribute only |
| Legacy attributes, init rerun | Lines rewritten, both drivers set | None |
| Union and legacy lines both present (older binary ran init) | Rewrite leaves duplicate union lines | None |
| Union attributes, no driver (fresh clone) | Built-in union; rebase accepted | None |

## 3. Decisions
- **M1 decides the attribute.** Codeberg is the canonical remote, so its result decides, not GitHub's.
- **All four paths stay together.** `docs/README.md` conflicts on concurrent study rows as often as the
  issue indexes do. If union stays, its silent joining of hand-written lines is an accepted, documented
  risk: nothing is lost, and the local driver still refuses.
- **Both attribute values stay accepted** by `runIssuesRebase` and init migration in either outcome,
  because existing repositories hold both.
- **Index freshness after a forge merge is out of scope.** No hook runs on the forge; see 1045 and 1055.

## 4. Milestones

### M1: Live forge canary (needs owner approval, creates public scratch repositories)
Create one throwaway repository each on Codeberg and GitHub. On `main`, commit
`union.md merge=union` in `.gitattributes`, plus `union.md` and `control.md` (no attribute) with the
same three-line content. Open four PRs from four branches, each adding a different line at the same
spot: two for `union.md`, two for `control.md`. Merge one PR of each pair, then read the other's state:

```bash
curl -s https://codeberg.org/api/v1/repos/<owner>/<repo>/pulls/<n> | jq .mergeable
gh pr view <n> -R <owner>/<repo> --json mergeable
```

`control.md` must report not mergeable, otherwise the run proves nothing. `union.md` reporting mergeable
means the forge honors union. GitHub can report `UNKNOWN` briefly; re-read it. Record both results in
this ticket and delete the repositories.

### M2-A: If Codeberg honors union, keep it
1. In the `default:` case of `runIssuesMergeDriverVersions` (`rebase.go:746`), when ancestor and other
   versions are present, apply `git merge-file --union` to `%A` instead of failing. This fixes Finding 2.
2. In `removeIssuesGitIntegration`, unset `merge.union.*` only when the value is the harnez command.
3. Change the docs to name exactly the forges M1 verified.

### M2-B: If Codeberg ignores union, revert the attribute (expected)
1. Make `issuesAttributesLines()` write `merge=harnez-issues-index` again, and treat union lines as the
   migration source. Configure only `merge.harnez-issues-index.*`.
2. On init and disable, remove `merge.union.*` only when the value is the harnez command.
3. Restore `.gitattributes` in this repository and run `harnez init -d .`.
4. Remove the forge claim from `docs/practices/IssueTracking.md`, `IssueTracking.lite.md`, and the root
   copy, and say that forge conflicts remain and are resolved by rebasing the topic branch.
5. Close this ticket with the M1 result, and file a follow-up only if the owner still wants forge
   conflict relief. 1055 covers index freshness, not mergeability.

### M3: Tests (both outcomes)
Assertions below use `<attr>` and `<driver>` for the values the chosen outcome installs.

Update existing tests:
- `internal/claude/init_test.go:409,414`: the foreign-tracker negative checks look only for
  `harnez-issues-index`, so they can no longer fail after `2a7c9bee`. Check `merge=union` and
  `merge.union.driver` too.
- `internal/claude/init_test.go:469,498,540`: assert the exact expected driver keys, including that
  disable leaves no harnez-owned `merge.union.*`.
- `cmd/harnez/issues_test.go:765`: replace the accept-either check with the exact `<attr>` line.
- `cmd/harnez/issues_index_init_merge_e2e_test.go:47-59`: pin `<driver>` and `<attr>`.

Add new tests:
- `internal/claude/init_test.go`: legacy-to-current migration is idempotent; a mixed file ends with one
  line per path and keeps unrelated lines; a user-owned `merge.union.driver` survives install and disable.
- `internal/issues/maintenance/rebase_test.go`: a table for `runIssuesRebase` integration detection:
  union without driver, legacy with driver, legacy without driver (error), unspecified (error).
- `cmd/harnez/issues_index_init_merge_e2e_test.go`: a user `CHANGELOG.md merge=union` merges cleanly
  after `harnez init` with both sides' lines kept. Finding 2's regression test.

### Acceptance criteria
- M1 results for both forges are recorded here.
- No forge claim remains in the docs beyond what M1 verified.
- A `merge=union` path outside the four generated files merges cleanly after `harnez init`.
- All repository states in the compatibility table have test coverage and a defined behavior.

## 5. Verification

```bash
make test-pkg PKG=./internal/claude/...
make test-pkg PKG=./internal/issues/maintenance/...
go test ./cmd/harnez -count=1 -run 'TestInitInstallsIssueMergeDriverEndToEnd|TestIssues'
make lint
make test-q1
make install
harnez init -d .
git check-attr merge -- issues/README.md issues/BACKLOG.md issues/CLOSED.md docs/README.md
git config --local --get-regexp '^merge\.'
harnez issues lint -d .
```

The doc sync check `TestRootDocCopiesMatchSources` must pass after editing `docs/practices/IssueTracking*.md`.
