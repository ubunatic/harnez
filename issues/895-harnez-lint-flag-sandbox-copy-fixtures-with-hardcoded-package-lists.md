# 895 — harnez lint: flag sandbox copy fixtures with hardcoded package lists

**Status**: Closed — superseded by 907: owner chose to remove copy fixtures (import, sunset, or container test) instead of linting their copy lists
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 889 (Canary guideline for sandbox copy fixtures, `docs/Canary.md` "Sandbox Copy Fixtures")

---

## 1. Problem & Motivation
A test helper that copies part of the repo into a temp module keeps a hardcoded list of package directories. When a
package is extracted or added, the fixture does not copy it and the canary fails only in the new package. Ticket 889
documents the rule (discover packages, do not enumerate them), but nothing checks it. Agents extracting packages in
lean sprints still hit this failure.

## 2. Technical Specification / Findings
- Detection idea: in `_test.go` files, flag a `filepath.Glob` or copy loop whose pattern names a fixed list of
  package directories (for example several literal `internal/<pkg>` strings), rather than a single glob or walk over
  the package tree.
- Open question for the implementer: whether `harnez lint` already has a check hook for test files, and whether a
  heuristic on literal path strings is precise enough to avoid false positives on fixtures that legitimately copy one
  named file.
- Report only (warning), not an error, until the heuristic is shown to have no false positives on this repo.

## 3. Implementation & Verification Plan
1. Find the lint entry point and its existing check list (`harnez lint`, `internal/` lint code).
2. Add a check for hardcoded package-path lists in sandbox copy helpers, with a reproduction test: a fixture with a
   fixed list of directories is flagged, and a fixture that globs the tree is not.
3. Run the check on this repo; it must report nothing or only real findings.
4. Verify: `make test-q1` passes, and `harnez lint` output names the file and line of a flagged fixture.
