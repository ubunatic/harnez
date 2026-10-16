# 796 — TestRunRate_LatencyBudget fails under -race

**Status**: Closed — raceEnabled const in cmd/harnez; latency budget x4 under -race
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
`go test -race ./cmd/harnez` failed TestRunRate_LatencyBudget: "cold
end-to-end runRate took 70ms, want sub-50ms". Pure timing budget; race
instrumentation slows SQLite and file I/O.

## 2. Technical Specification / Findings
- Same class as 793 (internal/usage). Test files cannot be shared across
  packages, so cmd/harnez needs its own `raceEnabled` const.

## 3. Implementation & Verification Plan
- cmd/harnez/race_test.go / norace_test.go: `raceEnabled` (build tags race / !race).
- TestRunRate_LatencyBudget budget is 50ms, x4 when raceEnabled.
- Verify: `go test -race -count=5 -run TestRunRate_LatencyBudget ./cmd/harnez`.
