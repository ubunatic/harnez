# 811 — test-short tier: go test -short ./cmd/harnez under 10s

**Status**: Closed — resolved in 3d1626f4; go test -short ./cmd/harnez 53.6s -> 8.3s, internal/subagent 10.8s -> 0.3s, full suite green
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: 808

---

## 1. Problem & Motivation
`go test -short ./cmd/harnez` takes ~40s, so `make test-short` is not a fast tier.

## 2. Technical Specification / Findings
Find slow tests (`go test -short -json`), mark subprocess/binary-building/integration tests full-tier with `testing.Short()` skips, or make them in-process. Target < 10s.

## 3. Implementation & Verification Plan
Report before/after per-package timings; full suite still runs every test.
