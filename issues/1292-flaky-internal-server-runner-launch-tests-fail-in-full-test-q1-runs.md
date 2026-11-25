# 1292 — Flaky internal/server runner-launch tests fail in full test-q1 runs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: issues/1291-test-suite-too-slow-find-and-remove-hot-spots-split-fast-and-full-tiers-better.md, docs/TestTiers.md

---

## 0. Goal
`/goal` Find why `TestRunnerLaunchBwrapSurvivesExtraFiles` and `TestRunnerLaunchBindChildPIDEnforcesProcessTree` (internal/server) fail only in full `HTO=0 make test-q1` runs, then fix the cause without weakening the assertions. Done when five consecutive full runs are green, or the cause is written down with a reproduction. Stop and report if the fix needs a design decision, or if the cause is outside this repo.

## 1. Problem & Motivation
Both tests fail in full `make test-q1` runs and pass when run alone (`go test -count=1 -run ...`) and with `-count=3`. Across the four full runs on 2026-11-25 (ticket commits 21ffce83 to c4744476), these two were the only failures. Each failed run means a developer or agent has to rerun the suite and judge by hand whether the failure is real, which slows every commit and hides real regressions.

## 2. Technical Specification / Findings
- Failure messages seen: `bwrap relayed = [], want [bwrap-1]` (SurvivesExtraFiles) and `bound child test failed: exit status 3` (BindChildPIDEnforcesProcessTree).
- Both tests run bwrap-based launches. Full runs run many packages in parallel, so bwrap start-up or child-PID timing under load is the likely cause. Not yet confirmed.
- Their runtime is short (about 0.01 to 0.03s), so a fixed wait or a race between the child and the relay is more likely than slow work.
- Unknown: whether the same failures occur on the parent commit of any recent change, and whether they also fail under `-p 1`.

## 3. Implementation & Verification Plan
1. Reproduce under load: run the two tests with `-count=20` while a parallel `go test ./...` runs, and record the failure rate.
2. Read the launch path and the test's wait logic. Replace any fixed budget with a ready signal from the child, per `docs/TestTiers.md` (Timing Rules).
3. Keep the assertions: relayed files and the process-tree check must still be checked.
4. Verify: five consecutive full `make test-q1` runs are green, and the targeted run passes under load.
