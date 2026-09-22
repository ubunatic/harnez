# 289 — harnez init go.work reconciliation hard-fails on testdata/fixture go.mod files

**Status**: Closed — already fixed: ignoredModuleScanDir skips testdata (d602096), regression test gowork_test.go:178
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [issues/287](287-go-work-ambient-workspace-file-breaks-unrelated-tooling-in-sibling-repos-general-fix-needed.md) (adds the `harnez init` go.work reconciliation this ticket found a gap in), `internal/claude/gowork.go` (`findGoModules`, `ignoredModuleScanDir`)

---

## 1. Problem & Motivation

`findGoModules` (`internal/claude/gowork.go:146-176`, added in b1e131a for
issue 287) walks the entire target repository tree looking for `go.mod`
files and treats every one it finds as a repository module that must be
included in a newly created local `go.work`. It only skips
`.git`, `.hg`, `.svn`, `node_modules`, `vendor` — it does not skip
`testdata`, even though shipping a `go.mod` fixture under `testdata/` is a
common, well-established Go convention for testing module-aware tools
(module parsers, linters, code generators, `go work`/`go mod` wrappers).

When such a fixture exists and its `go.mod` is intentionally malformed
(a very common shape for negative-path fixtures), `go work init` fails
hard on it, and `reconcileGoWorkspace` propagates that error all the way
up through `RunInitWithIssuesGit`, aborting the entire `harnez init` run
— not just the workspace step — for a repo with a completely unrelated
Go-tooling test fixture.

Live repro (2026-09-09, from a fresh Sonnet review of b1e131a):

```
$ mkdir -p target/testdata/brokenmod && cat > target/testdata/brokenmod/go.mod <<'EOF'
this is not valid go.mod syntax !!!
EOF
$ cd target && go work init . ./testdata/brokenmod
go: errors parsing testdata/brokenmod/go.mod:
testdata/brokenmod/go.mod:1: unknown directive: this
```

`findGoModules` would include `./testdata/brokenmod` in `moduleArgs`
exactly like this, and `reconcileGoWorkspace` would return this error
from `go work init`, which `RunInitWithIssuesGit` (`internal/claude/init.go:449-455`)
returns unmodified — failing the whole `harnez init` invocation.

Even without a syntax error, a *valid* fixture `go.mod` under `testdata/`
would still be silently pulled into the generated `go.work` as a "project
module," which is not what a user asking for workspace isolation wants.

This did not appear in the current `harnez` repo itself (it has exactly
one `go.mod`, at the root) or trigger during review, but it is a
realistic hazard for any Go-tooling-shaped sibling repo under
`~/projects/.uman.toml` (issue 287's stated blast radius) the moment
`harnez init` is run against it.

## 2. Technical Specification / Findings

- `ignoredModuleScanDir` (`internal/claude/gowork.go:177-183`) has no
  entry for `testdata`, `_*`, or `.*`-prefixed directories (Go itself
  ignores `testdata` and any directory starting with `_` or `.` for
  package loading and `go build ./...`/`go test ./...` — this walker
  does not mirror that convention).
- No test in `gowork_test.go` exercises a repo containing a `go.mod`
  under `testdata/` (valid or invalid), so this gap has no regression
  coverage.

## 3. Implementation & Verification Plan

- Extend `ignoredModuleScanDir` (or the `WalkDir` callback directly) to
  skip `testdata` directories and directories prefixed with `_` or `.`,
  matching Go's own package-discovery conventions.
- Add a `gowork_test.go` case with a `go.mod` fixture under
  `testdata/` (both a syntactically invalid one and a valid-but-unrelated
  one) and assert `planGoWorkspace`/`reconcileGoWorkspace` neither treats
  it as a project module nor fails.
- Consider whether `reconcileGoWorkspace`'s error path should ever be
  fatal to the rest of `harnez init` (e.g. downgrade to a printed
  warning + skip, matching the "report the decision, don't create files
  prematurely" spirit of issue 287 §3.1) once the above is fixed, so a
  future unanticipated `go work init` failure degrades gracefully
  instead of aborting init.

## 4. Acceptance Criteria

- [ ] `go.mod` fixtures under `testdata/` (and `_`/`.`-prefixed dirs) are
      excluded from `findGoModules`'s module discovery.
- [ ] A regression test covers a repo with a `testdata/.../go.mod`
      fixture, including an intentionally malformed one, and confirms
      `harnez init`'s go.work step no longer fails or misclassifies it.
- [ ] Decide and document whether a future workspace-probe failure should
      abort the whole `harnez init` run or degrade to a warning.
