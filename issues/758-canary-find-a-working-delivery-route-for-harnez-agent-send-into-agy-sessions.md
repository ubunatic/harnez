# 758 — Canary: find a working delivery route for harnez agent send into AGY sessions

**Status**: Open — Reopened: explore harnez-agy / agy-meter-run supervisory queueing mechanism for prompt injection like codex queue
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [AGY delivery route canary](../docs/studies/2026-10-08-agy-delivery-route-canary.md), issues/656-harnez-agent-send-opt-in-messaging-to-detached-agents-after-host-session-agent-tracking.md, issues/680-probe-codex-cross-task-messaging-for-agent-send.md, issues/740-claude-plugin-mvp-harnez-agent-send-delivers-via-claude-sendmessage-claude-to-claude.md

---

/goal Prove, with probe evidence, which route can deliver a message into a running or idle AGY session, and record it in docs/studies/, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
`harnez agent send` delivers to Claude sessions (issue 740, harnez channel) and queues to Codex
threads (issue 680, `codex queue`). It has no AGY route. Ticket 656 M3 asks for a delivery canary
per provider; this is the AGY one. M4 (`harnez agent send` for AGY) waits on its result.

## 2. Technical Specification / Findings
Checked against code on 2026-10-08:
- `cmd/harnez/agent_send.go` (`resolveAgentSendTarget`) resolves only `provider=claude` channel
  endpoints and `provider=codex` threads. AGY targets are not listed and not resolvable.
- AGY hooks (`internal/agy/hooks.go`, `harnez hook agy` / `agy-post` in `cmd/harnez/hook.go`)
  register only PreToolUse and PostToolUse. PreToolUse prints `{"decision":"allow"}`; neither hook
  injects context today.
- Resume exists for harnez-started AGY workers: `AgyDriver.Resume` (`internal/subagent/agy.go:129`)
  runs `agy --conversation <id> --add-dir <dir> --output-format json -p <prompt>`.
- Interactive sessions started by harnez record the AGY conversation ID
  (`watchInteractiveProviderID`, `internal/subagent/interactive.go:171`), so name → ID is
  resolvable for those. Sessions started outside harnez have no known mapping yet.
- Canary result (Phase 1): **No native live route in standalone agy; resume-only**. Print-mode `--conversation` appends to the same history, but standalone `agy` has no queue/inbox CLI command, and lifecycle hooks only permit tool argument rewriting rather than context/message injection. See the [AGY delivery route canary](../docs/studies/2026-10-08-agy-delivery-route-canary.md).
- Architectural opportunity (Phase 2): **Supervised AGY execution via `harnez-agy` / `agy-meter-run`**.
  - Users and Harnez launch AGY via the managed launcher `harnez-agy` (or `harnez-agy --meter`, `harnez agy-meter-run`, `harnez agent start -i`).
  - Because `harnez` is the parent process / supervisor of `agy`, Harnez can maintain a local session FIFO queue / mailbox (e.g. `~/.harnez/sessions/<id>/inbox`).
  - This provides a clean provider-analogue to `codex queue`: external callers use `harnez agent send` to append messages to the queue, while the `harnez-agy` supervisor manages prompt injection into the supervised AGY process.

## 3. Canary & Prototype Plan
Phase 1 (Done): Probed standalone AGY CLI capabilities, lifecycle hooks, and resume. Documented in `docs/studies/2026-10-08-agy-delivery-route-canary.md`.

Phase 2 (Current Focus): **Supervised Queue Injection via `harnez-agy`**
1. **R5 — Harnez Supervisor Queue (`harnez-agy`)**:
   - Define mailbox / queue mechanism managed by `harnez-agy` (or `agymeter`).
   - Probe injection methods from supervisor into the supervised session:
     - Method A: Supervisor feeds queued prompt lines into child process stdin / interactive PTY when idle.
     - Method B: In `--meter` mode, supervisor intercepts the API conversation stream on `daily-cloudcode-pa.googleapis.com` and appends prompt turns.
     - Method C: Turn boundary injection when child process yields or requests next input.
2. Verify discovery:
   - Ensure `harnez-agy` registers its session ID / PID in a local registry so `harnez agent send` and `harnez channel list` can target it.
3. Test delivery:
   - Prove that `harnez agent send --name <agy-session> "..."` queues the message and the supervised session receives and processes it.
