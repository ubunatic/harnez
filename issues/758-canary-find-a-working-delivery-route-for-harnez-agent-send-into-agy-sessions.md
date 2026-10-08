# 758 — Canary: find a working delivery route for harnez agent send into AGY sessions

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: issues/656-harnez-agent-send-opt-in-messaging-to-detached-agents-after-host-session-agent-tracking.md, issues/680-probe-codex-cross-task-messaging-for-agent-send.md, issues/740-claude-plugin-mvp-harnez-agent-send-delivers-via-claude-sendmessage-claude-to-claude.md

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

## 3. Canary Plan
Setup: two throwaway AGY sessions in a scratch dir: T1 interactive (`harnez agent start`
interactive, note its conversation ID from `harnez agent list`), T2 a detached worker. Each message
is harmless and unique, e.g. `canary-758-<route>-<n>: reply "ack <n>"`, and names the sender.

Per route, record in a table: command run, target state (running mid-turn, idle at prompt,
detached), whether the message appears in the session, latency, whether the sender gets a receipt
or reply, side effects (new conversation, forked history, lost turn), and access needed.

| Route | Probe | Pass | Fail |
|---|---|---|---|
| R1 resume | `agy --conversation <T1 id> -p "<msg>"` while T1 TUI is idle, then while mid-turn; same on T2 via `harnez agent resume --name T2 "<msg>"` | T1's open TUI shows and answers the message without restart, and no second conversation is created | message lands only in a separate process or forks history; T1 TUI never sees it |
| R2 hook injection | temporary PreToolUse/PostToolUse hook that reads a mailbox file and returns the message as hook output; trigger a tool call in T1 | T1's next turn acts on the message text | AGY ignores hook output other than `decision`, or the schema has no context field |
| R3 native messaging | check `agy --help` and AGY docs for a send/queue/inbox command or tool (AGY analogue of `codex queue`) | a command delivers into the running T1 by conversation ID | no such command exists (record the help output as evidence) |
| R4 other | anything found during R1–R3 (e.g. an MCP tool exposed to AGY that polls a harnez mailbox) | as R1 | as R1 |

Also record: whether `agy` lists conversations non-interactively (needed for name → ID outside
harnez), and whether the recipient can tell who sent the message.

Done when: each route has a pass/fail row with evidence, a recommended route is named (or "no
live route; resume-only"), and the result is written to `docs/studies/` and linked here. Revert any
temporary hook. Do not implement AGY in `harnez agent send`; that is 656 M4.
