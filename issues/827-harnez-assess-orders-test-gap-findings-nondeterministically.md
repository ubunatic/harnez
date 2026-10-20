# 827 — harnez assess orders test-gap findings nondeterministically

**Status**: Closed — Fixed map-derived warning ordering and tied summaries in 8651919a; canonical full directory keys, 50-run text/JSON regression, loom/cati 0/5 differences, lint and requested tests pass
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

## Delivery

- Fix: `8651919a` (`fix(assess): stabilize directory findings and tied summaries (#827)`).
- Root cause: directory test-gap warnings were appended by ranging over a Go map. Sort the
  full target-relative directory keys before emission; normalize separators to `/`. The walk is
  sequential and already retains full paths, without basename deduplication. Category summaries
  also needed a name tie-breaker for equal line counts; sibling stacked token bars needed a key
  tie-breaker for equal token counts. Hotspot findings, RAMP artifacts, and document summaries
  already have deterministic ordering.
- Regression fixture: 50 in-process assessments assert exact warning paths/counts and byte-identical
  text and JSON. Includes root, nested directories, `assets/book` and
  `website/book/assets/book` with distinct counts, a tested sibling named `book`, a directory below
  the warning threshold, and equal-sized language groups. Another 50 iterations check token-bar ties.
- Host verification, 2026-10-20: five runs per repository and format, comparing each output to run 1.
  Loom text: **3/5 -> 0/5** differ; JSON: **1/5 -> 0/5**.
  Cati text: **4/5 -> 0/5** differ; JSON: **3/5 -> 0/5**.
- The historical book-path alternation was not independently reproducible: neither book directory
  exists in the current loom checkout. The fixture verifies both full paths remain distinct and
  stable; no path truncation or alias-merging mechanism was found.
- Passed: `make install`, `make lint` (warnings only),
  `go test ./internal/assess/ ./cmd/harnez/`, and `make test-short`.
  Existing goldens remain unchanged. Reviewed the final diff and assertions locally as a leaf worker.
