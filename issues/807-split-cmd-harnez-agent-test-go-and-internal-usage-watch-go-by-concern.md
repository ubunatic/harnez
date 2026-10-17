# 807 — Split cmd/harnez/agent_test.go and internal/usage/watch.go by concern

**Status**: Closed — resolved in d600dad9 (agent_test.go → 12 files) and 7920fc29 (watch.go → 9 files); move-only verified by sorted-decl diff, full suite and make container-test green
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: 276, 057

---

## 1. Problem & Motivation
`cmd/harnez/agent_test.go` (~4200 lines) and `internal/usage/watch.go` (~3250 lines) are the worst hotspots; finding the right test or render path costs agents large reads.

## 2. Technical Specification / Findings
Split by concern into focused files in the same package (pure moves, no behaviour change, no API change).

## 3. Implementation & Verification Plan
`go test ./cmd/harnez ./internal/usage` green after every move; `go vet`; diff is move-only.
