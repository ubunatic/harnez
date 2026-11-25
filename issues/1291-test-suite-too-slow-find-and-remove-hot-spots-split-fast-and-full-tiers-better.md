# 1291 — Test suite too slow: find and remove hot spots, split fast and full tiers better

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Performance
**Related**: docs/TestTiers.md, issues/1288-slow-testagentselftestgeneratedsessionacrossshells-takes-31s.md (merged here), issues/881-test-suite-catalog-cmd-harnez-dominates-the-run-lighter-and-bounded-tests.md, issues/884-split-cmd-harnez-move-command-logic-and-tests-into-internal-packages.md, issues/811-test-short-tier-go-test-short-cmd-harnez-under-10s.md

---

## 0. Goal
`/goal` Cut the wall time of `make test-short` and `make test` by finding the slowest tests and removing their hot spots, and make sure every slow test sits in the full tier, not the short tier. Done when all acceptance criteria in section 4 are checked. Stop and report if a fix needs a design decision from the owner, or if a change makes a test weaker without an equivalent check.

## 1. Problem & Motivation
The full suite is slow, and the fast tier is not fast enough to run before every commit. Earlier work moved `cmd/harnez` logic into `internal/` and tightened a few tests (881, 884, 811). That work was never measured as one pass against the tier split in `docs/TestTiers.md`. Each run pays for the slow cases again, so every developer and agent waits on them. One test alone (H1 below) takes about a quarter of the `cmd/harnez` package time. That pushes `go test ./cmd/harnez` close to the 120s timeout used in ad-hoc runs.

### Expected
- `make test-short` is fast enough to run before every commit. It runs no binary builds and no PTY, media or subprocess tests outside the exceptions listed in `docs/TestTiers.md`.
- `make test` is shorter than the baseline. Every slow test in it has a measured reason to be slow.
- No single test dominates a package: H1 drops from about 31s to under 5s.
- Coverage stays the same. Every check that leaves the short tier still runs in the full tier.

## 2. Technical Specification / Findings

### Hot spots
- **H1 `TestAgentSelftestGeneratedSessionAcrossShells`** (`cmd/harnez/agent_selftest_test.go:19`, merged from 1288).
  - Measured: 31.5s in a full `go test -json ./...` run (harnez 7cba5775 tree, TMPDIR unset). It was the top entry in the `cmd/harnez` per-test list, where the next slowest test took about 4s. The package took about 120s, and its top-level tests summed to about 119s serially.
  - Root cause (confirmed by reading the code; not timed separately): the test runs the background command that `--step hello` prints, and that command has no `--duration` flag. The background step therefore sleeps the spec default `selftest.background_duration_ms: 30000` (`spec/agent.yaml:93`, read through `subagent.SelftestBackgroundDuration`). That accounts for about 30s of the 31.5s. The `go build -o <tmp>/harnez .` step (line 28) costs the rest. 1288 guessed the build was the main cost, and the code shows it is not.
  - Fix direction: run the printed background command with a short duration. The sibling test `TestAgentSelftestPrintedCommandsAcrossShells` already appends `--duration 1ms` (line 184). Keep the check that the printed commands carry the generated session. Optionally share one built CLI binary between the two `*AcrossShells` tests, since each runs its own `go build`.
  - Target: under 5s on a warm build cache, with no loss of shell-session coverage.
- **H2 `internal/server` runner-launch tests** (`TestRunnerLaunchBwrapSurvivesExtraFiles`, `TestRunnerLaunchBindChildPIDEnforcesProcessTree`). They failed in full `make test-q1` runs but pass when run alone, so the failures depend on load. Check their runtime and flakiness, since they add to the full tier and may hide real slowness.
- **H3 onward**: unknown until the baseline below is measured.

### Tier split
- `docs/TestTiers.md` says the short tier skips PTY, media, and subprocess tests with `testing.Short()`. The real split may not match this. Some slow tests may run in the short tier, and some fast ones may be skipped in it. H1 builds a binary and runs subprocesses, but it has no `testing.Short()` skip.
- Earlier measurements, from closed tickets on an old tree (re-measure before relying on them): `cmd/harnez` took 88.8s to 151.8s before 881/884. `go test -short ./cmd/harnez` went from 53.6s to 8.3s after 811.
- Unknown: the current per-package and per-test timings, and how much of each tier is compile time versus test time.

## 3. Implementation & Verification Plan
1. Baseline: run `go test -json ./...` with the load average recorded, and time `make test-short` and `make test`. Record the 20 slowest tests per package, and the compile time versus test time split for each tier.
2. Fix H1 first (section 2), then re-time it alone with `go test -run TestAgentSelftestGeneratedSessionAcrossShells -v ./cmd/harnez`.
3. Classify every other test in the top 20. Each one gets one outcome: move it to the full tier (a `testing.Short()` skip with a reason), speed it up (remove a repeated build, a fixed sleep, or a wait on a wall-clock budget), or leave it with a written reason. Follow the subprocess-binary and timing rules in `docs/TestTiers.md`.
4. Keep assertions and coverage. Do not weaken a test to make it faster. If a check moves tier, the full tier must still run it.
5. Check H2 for load-dependent failures. Fix them, or file a separate ticket with the failing output.
6. Re-measure both tiers and report the before and after numbers with the load average. Update `docs/TestTiers.md` if the tier rules change.

## 4. Acceptance Criteria
- [ ] Baseline recorded in this ticket: `make test-short` and `make test` wall times, the load average, and the 20 slowest tests per package from `go test -json ./...`.
- [ ] H1 runs in under 5s alone on a warm build cache, and the `cmd/harnez` package time falls by about the same amount (about 25s or more).
- [ ] H1 still runs the hello, background, confirm, confirm-running and verify steps through `/bin/sh`, and still asserts that the printed commands carry the generated session and that the final state is complete.
- [ ] Every test in the re-measured top 20 has a recorded outcome in this ticket: sped up, moved to the full tier with a `testing.Short()` reason, or left with a written reason.
- [ ] No test in the short tier builds a binary or runs a PTY, media or subprocess step, unless `docs/TestTiers.md` lists it as an exception.
- [ ] H2 either passes in 3 consecutive full `make test-q1` runs, or has its own ticket with the failing output.
- [ ] No assertion was removed or weakened. The diff review confirms that every moved check still runs in the full tier.
- [ ] After numbers recorded next to the baseline, and both tier wall times are lower than the baseline.
- [ ] `docs/TestTiers.md` matches the final tier split.

## 5. Delivery
- One commit per hot spot or per tier change, each referencing 1291, so a slowdown can be bisected.
- Before and after timings go in section 2 of this ticket, not in commit messages only.
- A fresh reviewer subagent checks test rigour (no weakened assertions) before the final commit, as the repo rules require for larger changes.
- Close with `harnez issues close -d . 1291 "<summary>"` once all acceptance criteria are checked.
