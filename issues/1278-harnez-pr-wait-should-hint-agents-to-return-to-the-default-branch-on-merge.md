# 1278 — harnez pr wait should hint agents to return to the default branch on merge

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Enhancement
**Related**: `cmd/harnez/pr.go`, `internal/forge/`

---

## Goal

`/goal` Emit a short, non-intrusive hint on successful `harnez pr wait` completion reminding agents to switch back to and fast-forward the local default branch (and clean up the merged topic branch) when appropriate for their setup, without breaking `--json` output, and verify with unit tests. Stop and report if blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

When `harnez pr wait` reports that a PR has merged, agents often remain on the merged topic branch and start the next task or issue from it instead of returning to the local default branch (`main`). A brief hint in the human-readable success output (e.g., suggesting returning to the default branch locally if it makes sense in their setup) helps prevent accidental work on stale topic branches.

## 2. Scope & Notes

- Re-verify live `harnez pr wait` output formatting and `--json` contract before implementing.
- Show the hint only on successful merge (exit 0) in human-readable output, keeping `--json` output clean/machine-parseable.
- Phrase the hint conditionally ("if that makes sense in your setup") so it stays accurate across worktrees, stacked PRs, and team/solo workflows.

## 3. Verification

- Unit tests covering `harnez pr wait` success output (hint present in text mode, absent or structured cleanly in `--json` mode) and non-merge exits (no hint).
- `HTO=0 make test-q1` passes.
