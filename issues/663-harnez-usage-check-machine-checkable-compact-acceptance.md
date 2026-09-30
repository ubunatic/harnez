# 663 — harnez usage --check: machine-checkable compact acceptance

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

Status: open · Priority: P2 · Severity: low · Category: feature

## Problem
Tickets 650 and 657 were closed on eyeballed or claimed compact output while Claude and Codex rows
were missing or varied between runs. Hosts and devs need one command that fails when compact is
incomplete.

## Goal
/goal `harnez usage --check` renders the compact projection from the store (no TUI) and exits
non-zero, naming the provider and reason, unless every configured/installed provider (Claude,
Codex, AGY pools) has a row with a percentage and reset time for each expected window, or an
explicit error row; `--json` prints the per-provider result. Fast (store only, no live fetch).
Tests cover pass, missing provider, missing window, and error row. Document it in
`docs/UsageCollection.md` and as the acceptance check in `docs/commands/lean-sprint.md` for usage
tickets. Stop and report when blocked on a user decision.

Related: 650, 657, 660, 659.
