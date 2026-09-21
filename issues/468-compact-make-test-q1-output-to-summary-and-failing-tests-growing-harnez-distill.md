# 468 — Compact `make test-q1` output to summary and failing tests, growing `harnez distill`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Ergonomics / Distillation
**Related**: [066](066-native-go-command-output-distillation.md), [178](178-distill-smart-mode-error-pattern-preservation.md), `Makefile` (`test-q1`), `internal/distill/`

---

## Problem

`make test-q1` prints one line per package plus all logs of failing tests. Quota-1 allows one run
per change, so hosts truncate the output (as in the 462 review, where only the first 20 lines
were read) and can miss a late failure, with no second run to check.

## /goal

`make test-q1` shows only a summary (packages ok/failed, counts, duration) and the failing tests
with their relevant output; the full output goes to a log file whose path is printed. Build it on
`harnez distill`, extending distill where needed (for example a Go-test mode). This is a good
use case to grow distill, which has been neglected and never fully used.

## Notes

- If the compacted output is still too long, treat that as a high-priority defect of the compaction, not of the caller.
- A failure must never be hidden: exit code and the failing-test list are always shown.
- Coordinate with 178 (smart mode / error-pattern preservation).
- Re-verify against live code before starting.
