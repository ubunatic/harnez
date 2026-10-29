# 1018 — reconcile follow-ups: canary on a real merge, share renumbering with mv, test --check with --pull

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1016, 1015, 1017

---

## 1. Problem & Motivation
`harnez issues reconcile` (1016) shipped with `--fetch`/`--pull`. Deferred pieces remain.

## 2. Technical Specification / Findings
- No canary on a real `git merge` with the index hook and merge driver active; tests use synthetic repos.
- Renumbering in `internal/issues/maintenance/reconcile.go` duplicates the logic of `runIssuesMv` in `cmd/harnez/issues.go`; 1016 asked for shared code (also with `issues rebase`, 1017).
- The `--check`/`--dry-run` with `--pull` rejection in `cmd/harnez/issues.go` has no test.
- Hook-failure-after-merge path in `--pull` (merge commit made, duplicate-number hook fails) is untested.
- The hidden merge-hook option was rejected on purpose; `--pull` is the explicit path.

## 3. Implementation & Verification Plan
1. Canary: real merge with colliding ticket numbers, run `reconcile --pull`, assert lint clean.
2. Extract one renumber function used by `mv`, `rebase` repair and `reconcile`.
3. Add cmd-level tests for the flag rejection and the hook-failure path.
