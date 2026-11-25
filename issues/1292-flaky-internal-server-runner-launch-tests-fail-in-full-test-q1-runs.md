# 1292 — Runner-launch tests fail under harnez exec (inherited broker FD)

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: issues/1291-test-suite-too-slow-find-and-remove-hot-spots-split-fast-and-full-tiers-better.md, docs/TestTiers.md

---

## 0. Goal
`/goal` Find why `TestRunnerLaunchBwrapSurvivesExtraFiles` and `TestRunnerLaunchBindChildPIDEnforcesProcessTree` (internal/server) fail in full `HTO=0 make test-q1` runs, then fix the cause without weakening the assertions. Done when a full `make test-q1` run is green.

## 1. Problem & Motivation
Both tests failed in every full `make test-q1` run on 2026-11-25 and passed when run directly. Each failed run meant a rerun and a manual judgment on whether the failure was real.

## 2. Findings (confirmed)
- Cause: `harnez exec` exports its own broker socket as `HARNEZ_BROKER_FD`. The tests inherited it. `prepareForwardedRunnerBroker` (internal/server/runner_launch.go) forwarded that socket to the test child, so the child relayed to the outer broker instead of the test's broker. Result: `bwrap relayed = []` and `bound child test failed: exit status 3`.
- Evidence: the package passed when run directly (3 of 3). It failed through `harnez exec`, with or without `--quota-1`. Clearing `HARNEZ_BROKER_FD` in the two tests made them pass under `harnez exec`. The quota variable and the process group were ruled out: each alone passed.
- Not a load or timing problem: the earlier "load-dependent" theory was wrong.

## 3. Resolution
- Both tests now clear the inherited variable with `t.Setenv(cfg.RunnerChannel.FDEnv, "")` before they start their own broker. Assertions are unchanged.
- Fix commit: b86c6aab (refs 1292).
- Verified: `harnez exec --quota-1 -- go test ./internal/server/` passes; full `HTO=0 make test-q1` exits 0.

## 4. Decided: keep forwarding the broker to children
Question: should `harnez exec` keep handing its broker socket (`HARNEZ_BROKER_FD`) to every child process?
Decision (decide mode, smart advisor, 2026-11-25): keep it, no product change. `harnez exec` cannot tell a harnez runner from `make` or `go test` that later calls `harnez` deeper in its tree. Dropping the broker for plain children would silently remove telemetry from those nested calls (ticket 1250). Forwarding adds no privilege: `init()` marks the inherited FD close-on-exec, so it is passed only on purpose, and the relay binds to the child's process tree.
Follow-up (optional, not started): a `TestMain` in `internal/server` that unsets the variable, replacing the per-test clears. Tests that host their own broker need that clear regardless.
