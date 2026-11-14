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

## Decision
- Record a tested-tree snapshot in `quota1.RunRecord`: `TestedHead` (`git rev-parse HEAD`, when inside a git repo) plus `TestedDirty` content hashes for eligible dirty tracked/staged/untracked files present when the test run started.
- Compare current eligible source content against that snapshot in **both** `ChangesSinceLastRunAfter` (`warnQuota1Changes`) and `CheckAndRecord` (the Quota-1 rerun gate), so a branch switch/fast-forward to the tested code neither fires a false warning nor falsely unlocks a duplicate test rerun, while editing one eligible file warns/unlocks for that file only.
- Fall back to the existing mtime scan when `TestedHead` is empty (older state files or non-git directories in unit tests) so backwards compatibility and non-git tests remain intact.

## Pre-Work (Scout findings)
- `internal/quota1/quota.go:17` (`RunRecord`), `:183` (writes), `:142` (`ChangesSinceLastRunAfter`), `:419` (`CheckAndRecord`): add tested-tree snapshot fields (`TestedHead string`, `TestedDirty map[string]string` with SHA-256 hex or `""` for deleted/missing) and shared helper computing eligible source file differences against the snapshot (using `git diff --name-only <TestedHead>` + current dirty/untracked eligible files vs `TestedDirty`, filtering by the same eligible-source rules).
- `cmd/harnez/agent_run.go:269` (`warnQuota1Changes`), `:513`, `:916` (`quotaTurnStarted`): `warnQuota1Changes` already delegates to `quota1.ChangesSinceLastRunAfter`; check whether `turnStarted` cutoff needs any adjustment alongside snapshot diffing (snapshot comparison ensures pre-turn branch-switch files with unchanged content are never reported).
- `internal/quota1/quota_test.go:288` & `cmd/harnez/agent_quota_test.go:97`: add regression tests covering:
  1. branch switch + `git merge --ff-only` to the tested commit stays warning-free (`ChangesSinceLastRunAfter` empty) and blocks redundant `CheckAndRecord` rerun;
  2. changing one eligible source file afterward reports and unlocks for that file only;
  3. dirty tracked + untracked eligible files present during the test run don't trigger false warnings/reruns until modified.
- Verify with `go test -race -count=1 ./internal/quota1 ./cmd/harnez`.

### M2 Pre-Work (Reviewer findings on `bce91254`)
- `internal/quota1/snapshot.go`: guard `workingHash` (and `snapshotCandidates` / gitlink `git show` hash comparison) against directories, symlinks, and submodules/embedded `.git` directories (e.g. skip non-regular directories or hash symlink targets consistently via `os.Lstat` instead of failing `os.ReadFile` with `is a directory`).
- `internal/quota1/quota_test.go`: extend snapshot regression coverage to test:
  1. switching to a commit with different `code.go` content reports `code.go` (warning/unlock) and clears when returning to `TestedHead`;
  2. deleted committed file after the test run is reported, while a file already deleted before the run stays ignored until recreated/changed;
  3. directory/embedded-repo paths returned by git status/diff do not error `snapshotRun` or `changesSinceRecord`.
- Commit referencing `#1227` (or amend `bce91254` to include `(#1227)`).

**/goal** Make Quota-1 track tested tree state (`TestedHead` + dirty/untracked hashes) so branch switches/fast-forwards to identical tested code emit no untested-code warning and block redundant reruns, verified by tests, or stop and report when blocked on a user decision or denied permission.
