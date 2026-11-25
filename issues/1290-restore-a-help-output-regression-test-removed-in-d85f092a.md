# 1290 — Restore a help output regression test removed in d85f092a

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Infrastructure
**Related**: 1289

---

## 1. Problem & Motivation

Commit `d85f092a` ("chore(cmd): remove help golden test and testdata", 2026-11-24) removed the help
output golden test and `cmd/harnez/testdata/help.golden`. Its message says the golden file was no longer
kept in sync with the CLI, so the test was removed rather than regenerated. Nothing now fails when a
command's help text, flags or subcommands change unexpectedly.

## 2. Technical Specification / Findings

- The removed test compared the root help output with a checked-in file. It was not replaced.
- Other golden files remain in `cmd/harnez/testdata/` (`agent-picks.golden`, `agent-models-chart-80cols.golden`,
  `init_solo/`), so the golden pattern is still in use.
- The drift that caused the removal is not recorded, so it is unknown whether the golden or the help
  text was the stale side.

## 3. Implementation & Verification Plan

1. Decide whether a help snapshot is worth keeping. If yes, regenerate it with a documented update flag,
   check it in, and make the test assert only the root command names and the flags of the `agent` and
   `exec` commands, so it does not churn on every wording change.
2. If no, record the decision here and close the ticket with `--no-code`.
3. Verify with `go test ./cmd/harnez -run Help` and `make lint`.
