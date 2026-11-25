# 1291 — Test suite too slow: find and remove hot spots, split fast and full tiers better

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Performance
**Related**: docs/TestTiers.md, issues/1288-slow-testagentselftestgeneratedsessionacrossshells-takes-31s.md, issues/881-test-suite-catalog-cmd-harnez-dominates-the-run-lighter-and-bounded-tests.md, issues/884-split-cmd-harnez-move-command-logic-and-tests-into-internal-packages.md, issues/811-test-short-tier-go-test-short-cmd-harnez-under-10s.md

---

## 0. Goal
`/goal` Cut the wall time of `make test-short` and `make test` by finding the slowest tests and removing their hot spots, and make sure every slow test sits in the full tier, not the short tier. Done when the top-20 slowest tests are measured again, each one is either fast or skipped under `testing.Short()` with a reason, and both tiers are timed before and after. Stop and report if a fix needs a design decision from the owner, or if a change makes a test weaker without an equivalent check.

## 1. Problem & Motivation
The full suite is slow, and the fast tier is not fast enough to run before every commit. Earlier work moved `cmd/harnez` logic into `internal/` and tightened a few tests (881, 884, 811), but the work was not measured as one pass against the tier split in `docs/TestTiers.md`. Each run pays for the slow cases again, so every developer and agent waits on them.

## 2. Technical Specification / Findings
- Known hot spot: `TestAgentSelftestGeneratedSessionAcrossShells` (cmd/harnez) takes about 31s, with the `go build` step as the likely cause (1288, not yet measured).
- Earlier measurements (from closed tickets, old tree; re-measure before relying on them): `cmd/harnez` took 88.8s to 151.8s before 881/884; `go test -short ./cmd/harnez` went from 53.6s to 8.3s after 811.
- Two runner-launch tests in `internal/server` (`TestRunnerLaunchBwrapSurvivesExtraFiles`, `TestRunnerLaunchBindChildPIDEnforcesProcessTree`) failed in full `make test-q1` runs but pass alone. They are load-dependent. Their runtime and flakiness should be checked here, since they add to the full tier and may hide real slowness.
- `docs/TestTiers.md` says the short tier skips PTY, media, and subprocess tests with `testing.Short()`. The real split may not match this: some slow tests may run in the short tier, and some fast ones may be skipped in it.
- Unknown: the current per-package and per-test timings, and how much of each tier is compile time versus test time.

## 3. Implementation & Verification Plan
1. Measure the baseline: `go test -json` over `./...` with the load average recorded, plus `make test-short` and `make test` wall times. Record the 20 slowest tests per package.
2. Classify each slow test: move to the full tier (`testing.Short()` skip with a reason), speed up (remove a repeated build, a fixed sleep, or a wait on a wall-clock budget), or leave it. Follow the subprocess-binary and timing rules in `docs/TestTiers.md`.
3. Keep assertions and coverage. Do not weaken a test to make it faster; if a check moves tier, the full tier must still run it.
4. Check `internal/server` runner-launch tests for load-dependent failures. Fix or file a separate ticket with the failing output.
5. Re-measure both tiers and report before and after, with load average. Update `docs/TestTiers.md` if the tier rules change.
