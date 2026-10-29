# 1005 — Backfill historical closed-ticket outcomes flagged by status

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Issue tracking
**Related**: 577

---

## 1. Problem & Motivation
`harnez status` reports 116 historical closed tickets whose status gives only a commit reference instead of an outcome. This intentional warning from issue 577 helps readers understand what was delivered, but the volume obscures other status diagnostics.

## 2. Technical Specification / Findings
Issue 577 deliberately introduced the warning and backfilled only tickets 115 and 116. Verify the current warnings and ticket history before changing anything; commit references alone are not evidence of a specific outcome. Retain an honest unresolved or ambiguous description where the history does not establish one.

## 3. Implementation & Verification Plan
/goal Reduce historical outcome warnings with evidence-based summaries, or stop and report when blocked on a user decision or denied permission. On a loop, a hard blocker, or no progress, clear the goal and record the failure and resume point here.

Review ticket bodies and relevant commits, update justified close reasons without changing ticket disposition, regenerate indexes, and verify status and tracker lint. If bulk backfill is impractical, propose a bounded, non-misleading way to keep these warnings actionable without hiding new ones.
