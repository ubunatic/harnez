# 1113 — Jules driver: auto-answer one AWAITING_USER_FEEDBACK with a fixed finish-and-submit reply

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## Problem
Reported 2026-11-03 by the lmcoder session. Three of its five implement runs (149, 053 and 201) stopped in AWAITING_USER_FEEDBACK even though the prompt forbids questions. The /jules skill's "Asks back" section allows the host to reply by hand, but that costs a host round trip on every run.

## Goal
/goal The Jules driver can send one fixed reply when a run first waits for feedback ("Pick the smallest option, state it, finish and submit the PR now."), turned on by a spec flag, so runs finish without host help. Stop and report if Jules ignores the reply in live runs.

## Done when
- A `spec/agent.yaml` jules key turns the auto-reply on (spec-first; no Go default). The driver sends the reply at most once per session.
- A fake-client test covers one reply, no second reply, and the flag turned off.
- The skill's "Asks back" section says the first reply may already have been sent automatically.
