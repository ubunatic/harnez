# 757 — Show the current Codex session in harnez channel list

**Status**: Open — CODEX_THREAD_ID is not exported to child shells by the Codex CLI; must use process-level discovery via internal/subagent.DiscoverActiveCodexSessions
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Running `harnez channel list` does not show the Codex session that is currently using the command, making the active session hard to discover or coordinate.

## 2. Technical Specification / Findings
Observed from the current Codex session: this session is absent from the listing. The cause and intended channel registration/discovery mechanism are not yet known.

## 3. Implementation & Verification Plan
Investigate how `harnez channel list` discovers sessions and include the current Codex session when available. Verify listing behavior for Codex sessions and preserve behavior for other supported channels.

**/goal** Make `harnez channel list` show the active Codex session and verify session discovery, or stop and report if blocked on a user decision or denied permission.
