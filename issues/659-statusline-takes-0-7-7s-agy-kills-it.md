# 659 — statusline takes 0.7-7s, AGY kills it

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
AGY runs `harnez statusline --agent agy` and killed it (`signal: killed`, log
`~/.gemini/antigravity-cli/log/cli-20260930_163033.log`, 22:13:25, failure 1/30).
Measured on 2026-09-30: 0.67s, 2.2s, 3.0s, 7.2s, 7.2s per call (with and without stdin JSON),
while a dev ran `make test-q1`. A statusline must render in well under 1s.

## Goal
/goal The statusline renders from cached/stored data only (no live fetch, no blocking DB
lock), p95 < 300ms on this machine even under test load; find what blocks (DB lock after the
657 migration, live quota fetch, process scans) and fix it, or stop and report when blocked.
