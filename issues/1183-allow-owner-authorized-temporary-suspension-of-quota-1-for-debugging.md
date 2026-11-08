# 1183 — Allow owner-authorized temporary suspension of Quota-1 for debugging

**Status**: Closed — superseded by 1188 (/disable process quota-1)
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 1181

---

## Goal

/goal Provide an explicit, temporary owner-authorized way to suspend Quota-1 for agent debugging, verify that agents honor it and normal enforcement resumes, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record what was tried, why it failed, and the resume point.

## Problem

The repository and machine owner explicitly asked agents to suspend Quota-1 and run tests normally to debug a blocked sprint. Agents refused because current instructions reserve bypasses for human developers and CI, leaving no usable mechanism for the owner to authorize an agent. During #1181, nested Bubblewrap also prevented the Quota-1 suite from starting. The owner needs a way to direct debugging without agents treating the request as unauthorized.

## Acceptance

- Define an explicit owner-controlled suspension with clear scope, duration, and restoration to normal enforcement; avoid silently disabling the rule for unrelated sessions or future work.
- Ensure agent-facing instructions and runtime enforcement agree on how authorization is granted, recognized, and revoked; distinguish owner authorization from an agent self-authorizing.
- Verify normal enforcement, authorized debugging, expiry or restoration, and denial without authorization. Explain auditability and any trust limitations honestly.
- Recheck live rules, enforcement, and recent history before implementation; investigate existing bypass mechanisms rather than assuming their behavior.
