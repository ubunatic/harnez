# 662 — 30-minute dev budget: keep the default background limit, guide the host when a turn dies

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

Status: open · Priority: P2 · Severity: medium · Category: agent-workflow

## Problem
Claude Code kills a host background job after 30 minutes by default. On 2026-09-30 this killed
dev-654 mid-turn; `harnez agent wait` then returned only old messages, and the host recovered by
resuming with `timeout: 7200000`. 630 §7 records that workaround.

User decision (2026-09-30): keep the 30-minute limit. A developer that neither finishes nor
delivers intermediate results within 30 minutes signals a wrongly cut task: it needs splitting,
more research, or prep/cleanup elsewhere first. The limit is a useful budget, not a bug.

## Goal
/goal When a dev turn is killed at the host's background limit (or times out), harnez tells the
host what happened and what to do, and the guides treat 30 minutes as the dev budget:
- `harnez agent wait`/`resume`/`status` detect a turn whose host job died (process gone, no
  `[done]`) and print it plainly, with the last progress, the uncommitted diff stat, and next steps:
  ask the dev for a short state report, then split the ticket into milestones, add research or
  prep, or park the partial work as a patch; do not simply resume with a longer timeout;
- the lean-sprint doc and 630 §7 drop the `timeout: 7200000` advice and describe this budget
  and recovery flow instead; dev prompts ask for an intermediate commit or report within ~20 min;
- tests for the dead-turn detection and message.
Stop and report when blocked on a user decision.

Related: 630, 661 (truthful stop), 538.
