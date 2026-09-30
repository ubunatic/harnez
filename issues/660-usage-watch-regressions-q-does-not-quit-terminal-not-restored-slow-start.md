# 660 — usage --watch regressions: q does not quit, terminal not restored, slow start

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Describe the problem and why it matters.

## 2. Technical Specification / Findings
Record relevant technical details and findings.

## 3. Implementation & Verification Plan
Describe the implementation and how it will be verified.

Status: open · Priority: P1 · Severity: medium · Category: bug

## Problem
User report (2026-09-30), after the 650/657 compact work: `harnez usage --watch`
- `q` no longer quits,
- the terminal is not restored correctly on exit,
- initial loading takes much longer than before.

Suspects (unverified): recent `internal/usage/watch.go` changes in 5e1e6b2a (compact store
read seam), 325d0e53, 178421fc ("static compact sizing", rows beyond terminal height),
2d418fd4 (row ordering). A blocking store read or live collect before the first frame may
starve the key reader; see also 659 (statusline 0.7-7s).

## Goal
/goal `harnez usage --watch` shows its first frame fast (target < 1s, measure before/after),
`q` and Ctrl-C quit at once, and the terminal (alt screen, cursor, raw mode) is fully
restored on every exit path; add tests for the quit path and restore, and verify on the
installed binary. Bisect against the commits above if the cause is not obvious. Stop and
report when blocked on a user decision.
