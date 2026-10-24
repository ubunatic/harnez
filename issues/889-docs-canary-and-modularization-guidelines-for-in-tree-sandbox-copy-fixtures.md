# 889 — Docs: Canary and modularization guidelines for in-tree sandbox copy fixtures

**Status**: Closed — Added the Sandbox Copy Fixtures section to docs/other/Canary.md (source, commit 47b8a89d) and synced docs/Canary.md: discover packages by glob or walk instead of a fixed list, and audit copy patterns when extracting packages. make test-q1 finished with 0 '--- FAIL' lines (log /tmp/harnez-test-q1-894-s1.log). Automated detection of hardcoded lists is not built; it is filed as 895 (harnez lint check).
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/Canary.md`, `loom` issues 116/380

---

## 1. Problem & Motivation
In projects that use in-tree sandbox copy fixtures (such as canary tests or test isolation helpers copying a curated subset of repo packages into a temp Go module), adding or modularizing new packages breaks the canary/fixture tests unless the copy glob list is explicitly updated.
During lean sprints in `loom` (issues 116 and 380), subagent developers extracted new subpackages (`ansi`, `specdefaults`), only to encounter unexpected test failures because the in-tree sandbox fixture had a hardcoded list of package source globs to copy.

## 2. Technical Specification / Findings
- Update `docs/Canary.md` (and related agentic modularization guidance) to document the canary/sandbox fixture pattern.
- Explicitly note: when extracting, renaming, or introducing top-level packages or subpackages in a repository that employs module-copy fixtures, audit existing copy/sync patterns (e.g. `filepath.Glob` in test helpers) as part of the package extraction checklist.
- Consider whether `harnez lint` or harness checks can detect and report sandbox fixtures that copy a hardcoded list of Go package directories when new packages are created.

## 3. Implementation & Verification Plan
1. Edit `docs/Canary.md` in `harnez`.
2. Add a clear guideline under Canary test design regarding dynamic vs. static package discovery in sandbox fixtures.
3. Review and index with `harnez index -d ../harnez`.
