# 762 — Rollup: Review and audit cross-provider messaging and supervised AGY queueing changes

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/754-add-filtering-and-projection-flags-to-harnez-channel-list-live-names-ids-with-overview-first-design.md, issues/756-harnez-channel-list-should-list-only-active-agents-by-default-with-more-brevity.md, issues/757-show-the-current-codex-session-in-harnez-channel-list.md, issues/758-canary-find-a-working-delivery-route-for-harnez-agent-send-into-agy-sessions.md, docs/studies/2026-10-09-supervised-agy-prompt-queueing-and-cross-session-messaging.md, docs/AgentMessaging.md

---

## 1. Problem & Motivation

Across tickets 754, 756, 757, and 758, significant architectural additions were delivered to unify cross-provider session discovery and messaging across Claude, Codex, and Antigravity (AGY).

This collective rollup ticket tracks the review, audit, and follow-ups for the full set of changes before broad usage or release:
1. **`harnez channel list` UX & Filtering (#754, #756)**:
   - Added `--all`, `--names`, `--ids`, and `--json`.
   - Overview-first tabular default showing only active/live sessions.
2. **Process-Level Active Codex Discovery (#757)**:
   - Replaced fragile environment variable checks (`CODEX_THREAD_ID`) with process-level inspection (`subagent.DiscoverActiveCodexSessions`).
3. **Supervised AGY Prompt Queueing & `agent send` (#758 M0–M4)**:
   - Turn-boundary model stream observer in `internal/agymeter` (`functionCall` SSE detection).
   - Concurrency-safe `TurnState` tracking request, tool, and typing windows (`internal/subagent/agyturn.go`).
   - Durable FIFO disk-backed queue with atomic persistence (`internal/subagent/agyinbox.go`).
   - PTY supervisor harness in `harnez-agy --meter` and `harnez agent start -i` injecting bracketed paste (`\x1b[200~...\x1b[201~\r`) at idle turn boundaries.
   - Target resolution and delivery in `harnez agent send` and `harnez channel list`.
   - Bugfix for `session.ProcessPID` and control socket routing.

## 2. Review Checklist & Audit Points

- [ ] Verify multi-repo synchronization and doc copies (`docs/AgentMessaging.md`, etc.).
- [ ] Audit error handling and edge cases when supervised session dies while queue has pending messages.
- [ ] Confirm `spec/channel.yaml` schema settings (`quiet_ms`, `typing_guard_ms`, `max_message_bytes`) are fully utilized across all callers.
- [ ] Evaluate whether non-supervised AGY processes should output an advisory notice when started outside `harnez-agy --meter`.
- [ ] Run full release validation / smoke tests (`scripts/smoke-test.sh`).

## 3. Verification & Case Study Reference

See [`docs/studies/2026-10-09-supervised-agy-prompt-queueing-and-cross-session-messaging.md`](../docs/studies/2026-10-09-supervised-agy-prompt-queueing-and-cross-session-messaging.md) for full implementation history, candid failure analysis, and live verification evidence on `loom-agy`.
