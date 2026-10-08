# 758 — Canary: find a working delivery route for harnez agent send into AGY sessions

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: issues/656-harnez-agent-send-opt-in-messaging-to-detached-agents-after-host-session-agent-tracking.md, issues/680-probe-codex-cross-task-messaging-for-agent-send.md, issues/740-claude-plugin-mvp-harnez-agent-send-delivers-via-claude-sendmessage-claude-to-claude.md

---

/goal Prove, with probe evidence, which route can deliver a message into a running or idle AGY session, and record it in docs/studies/, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
`harnez agent send` works only for Claude sessions (issue 740). AGY sessions have no send path. Ticket 656 (M3) asks for a canary per provider, but no AGY probe exists. Without one, M4 (`harnez agent send` for AGY) cannot start.

## 2. Technical Specification / Findings
- No AGY send code path exists in `cmd/harnez` or `internal/channel` (checked on 2026-10-08).
- Candidate routes to probe, from ticket 656 M3: AGY's native messaging (if any), hook injection on the next prompt or tool event (AGY hooks live in `internal/agy`), and `harnez agent resume --name <session> "<msg>"`.
- Record for each route: whether it reaches a running session, an idle session, and a detached worker; whether the sender gets a receipt; and what access it needs.
- Open question: whether AGY exposes a stable session ID or name that `harnez agent send --name` can resolve.

## 3. Implementation & Verification Plan
- Re-verify the candidate routes against current code and AGY behaviour before probing.
- Probe each route on a throwaway AGY session. Keep the probe evidence and the commands used.
- Write the result to `docs/studies/` and link it from this ticket.
- Do not implement `harnez agent send` for AGY in this ticket. That is M4 of 656.
