# 496 — Clean up pre-existing gofmt drift across the repo

**Status**: Closed — gofmt -w on the 30 drifted files and a gofmt -l gate in make check (e5eb397)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: `docs/feedback/2026-09-22-component-system-design-doc-first.md`

## /goal

`gofmt -l .` is empty, and stays empty.

## Problem

30 Go files are not gofmt-clean (2026-09-22), across `cmd/harnez`, `internal/assess`,
`internal/claude`, `internal/usage`, `internal/release`, `internal/readcard`, `internal/mode`,
`internal/find`, `internal/fsutil`, `internal/markdown` and one canary script. The noise hides
real formatting drift in new changes, so every change has to filter `gofmt -l` by hand.

## Plan

1. One mechanical commit: `gofmt -w` on the listed files, no other changes; run the tests once.
2. Add a `gofmt -l` check to `make check` (fails when output is non-empty) so drift can't return.
