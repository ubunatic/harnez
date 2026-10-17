# 813 — Extract cohesive units from internal/claude apply.go and init.go

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: 807, 812

---

## 1. Problem & Motivation
`internal/claude/apply.go` (1985 lines) and `init.go` (1822 lines) are the next hotspots; `ApplyAllVariant` (479 lines, 8 params) and `RunInitWithVariant` (403 lines, 13 params) mix many concerns.

## 2. Technical Specification / Findings
Extract cohesive units (files and helper functions) with unit tests; no behaviour change.

## 3. Implementation & Verification Plan
`go test ./internal/claude` green, new unit tests for extracted helpers; hotspot numbers before/after.
