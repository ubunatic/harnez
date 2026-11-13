# 1227 — Quota-1 untested-code warning fires after a branch switch although the code equals the tested commit

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1225

---

## 1. Problem & Motivation

After an agent turn, `harnez agent` printed:

`harnez: 4 file(s) changed after the last quota-1 run at 2026-11-13T09:58:57Z:
cmd/harnez/agent_test.go, config.yaml, internal/claude/harnez_init_skill_test.go,
internal/claude/rules_init_test.go — code is untested, run make test-q1`

The code was tested. `make test-q1` had passed on the topic branch at commit `1ebc5fc3`; the
host then ran `git checkout main` and `git merge --ff-only` of that branch. `git diff 1ebc5fc3
HEAD -- '*.go' config.yaml` is empty. The four files named are exactly the non-Markdown files
that differed between the old `main` and the topic branch, so the branch switch rewrote them.

A false "untested" line teaches agents to ignore the warning, or to burn a test run that
Quota-1 then blocks because no source changed.

## 2. Technical Specification / Findings

- `warnQuota1Changes` (`cmd/harnez/agent_run.go`) calls `quota1.ChangesSinceLastRunAfter`
  (`internal/quota1/quota.go`), which lists files by modification time against the state file's
  timestamp. A checkout or fast-forward updates the modification time without changing what
  was tested.
- Unverified: whether the Quota-1 gate itself (`make test-q1` rerun check) has the same blind
  spot in the other direction, and why the `turnStarted` filter did not drop these files (the
  branch switch happened before the reviewer's turn started).

## 3. Implementation & Verification Plan

- Record the tested commit and a hash of the dirty tracked files with the state, or compare
  candidates with `git diff --quiet <tested commit> -- <file>` before reporting them.
- Test: run the quota state on a branch, switch to another branch and fast-forward to the same
  commit; the warning must stay silent. Then change one file; it must fire for that file only.
