# 827 — harnez assess orders test-gap findings nondeterministically

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 799

## Problem

Found while verifying 799 (2026-10-20). Repeated `harnez assess /home/uwe/projects/loom` runs give
different output: the "Directory X contains N code files but no test files" lines (text and `--json`)
change order and sometimes paths (`assets/book` vs `website/book/assets/book`). Pre-existing: the
v0.1.32-era binary differed in 4 of 5 runs, the current one in 3 of 5. The harnez repo itself is stable;
loom and cati are not. Likely map iteration or concurrent directory walking.

## Expected

Identical input gives byte-identical text and JSON output; findings sorted by a stable key; each
directory reported with one canonical repo-relative path.

## Acceptance

- Repro test runs the assessment on a fixture with several untested directories (nested too) many
  times and asserts identical output.
