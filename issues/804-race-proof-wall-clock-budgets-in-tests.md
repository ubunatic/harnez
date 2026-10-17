# 804 — Race-proof wall-clock budgets in tests

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 793, 796

---

## 1. Problem & Motivation
GitHub CI's `go test -race` step failed on a different fixed wall-clock budget
each run (`TestTier1_LatencyBudget`, run 39017732972: 1.23ms per call against
1ms). Race instrumentation slows code several-fold, so unscaled budgets turn
every slow runner into a red `main`.

## 2. Technical Specification / Findings
- Real latency budgets ("returns quickly", "doesn't wait for a slow
  dependency") get scaled: each package with them has `raceFactor` (1, or 4
  under `-race`) next to `raceEnabled` in `race_test.go`/`norace_test.go`.
- Where a scaled bound would reach the slow dependency it races against, the
  dependency gets longer instead (hook sound player 2s→10s, exec slow writer
  2s→10s, exec timeout children `sleep 1`→`sleep 5`).
- Ordering is proved with barriers, not clocks: the finder concurrency check in
  `TestSearchMergesDeduplicatesAndUsesArgumentSafeFakeFinders` now has the two
  finders wait for each other's marker file.
- Covered: cmd/harnez (agent_stop, exec, hook_sound, rate, statusline),
  internal/usage (passive), internal/telemetry, internal/sound, internal/index,
  internal/finder.

## 3. Implementation & Verification Plan
- Touched packages pass with and without `-race`, the race run limited to
  2 CPUs (`taskset -c 0,1`).
- GitHub CI passes on the branch through `workflow_dispatch`.
