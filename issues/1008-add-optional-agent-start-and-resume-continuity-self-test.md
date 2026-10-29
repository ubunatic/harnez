# 1008 — Add optional agent start and resume continuity self-test

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Self-test / agent lifecycle
**Related**: 665, 964, 784

---

## 1. Problem & Motivation
The background execution self-test does not prove an agent can start another agent via Harnez and resume the same session with its prior context.

## 2. Technical Specification / Findings
Add an optional step to `/harnez-selftest`: start a low-tier model through `harnez agent`, ask it to say a chosen word, resume the same session and ask what the word was, and verify the returned word. Report the selected model/provider and separate pass, failure and skip from the existing self-test. Do not count a refused launch, unavailable provider, or unverified response as a pass.

Issue 964 already proposes an optional dispatch smoke check in an extended skill; coordinate rather than duplicate it. Preserve host background-execution and leaf-worker restrictions, bound execution, and clean up only the test session.

## 3. Implementation & Verification Plan
/goal Add and verify an opt-in low-tier agent start/resume continuity check to the self-test skill, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and record the failure and resume point here.

Recheck the live skill and issue 964 before implementation. Cover response validation, same-session resumption, optional skip and failure reporting, timeout and cleanup; exercise a live provider when available without weakening the base self-test.
