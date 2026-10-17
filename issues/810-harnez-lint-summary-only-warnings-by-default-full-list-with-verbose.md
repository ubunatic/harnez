# 810 — harnez lint: summary-only warnings by default, full list with --verbose

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Tooling
**Related**: 806

---

## 1. Problem & Motivation
`make lint` prints all ~171 Go size warnings on every run, burying errors in CI logs.

## 2. Technical Specification / Findings
Text output lists every error, but only summarises warnings: count per rule plus the top 10 files by warning count. `--verbose` lists every finding; `--json` stays complete.

## 3. Implementation & Verification Plan
CLI tests for summary vs `--verbose`, errors always listed, JSON unchanged.
