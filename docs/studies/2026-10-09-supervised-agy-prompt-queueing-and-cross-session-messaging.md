# 2026-10-09 — Supervised AGY Prompt Queueing and Cross-Session Messaging Case Study

**Date**: 2026-10-09  
**Session Scope**: Tickets 754, 756, 757, and 758 (M0–M4)  
**Starting State**: `harnez channel list` dumped raw JSON across all discovered sessions; Codex thread discovery was un-gated and lacked liveness checks; AGY was completely disconnected from cross-agent messaging (`harnez agent send`).  
**Final Outcome**: Clean tabular overview with active-session defaults for `harnez channel list`; process-level active session detection for Codex; complete supervisory harness and idle bracketed-paste queueing for AGY sessions in `harnez-agy` / `harnez agent start -i`.

---

## 1. Executive Summary

This session resolved the final structural barrier in cross-provider agent-to-agent messaging. While Claude sessions communicate via the MCP channel plugin and Codex sessions accept messages via `codex queue`, Antigravity (AGY) sessions had zero native inbound queueing. Standalone `agy` lacks external messaging commands and its hooks cannot inject prompt context into ongoing conversations.

By leveraging Harnez's existing supervisory harness (`harnez-agy --meter` and `harnez agent start -i`), we turned the supervisor into an intelligent delivery proxy. The supervisor maintains a disk-backed FIFO queue, monitors turn boundaries via `agymeter` SSE stream parsing (`functionCall` detection), enforces a keystroke typing guard, and injects prompts as bracketed paste (`\x1b[200~ ... \x1b[201~\r`) directly into the PTY master when the session is safely idle.

The entire loop was designed through progressive advisory sessions (using `claude:opus:med`), scoped across five lean milestones (M0–M4), and verified live against a running user session (`loom-agy`), culminating in native execution of `harnez agent send --name loom-agy` and an immediate model reply.

---

## 2. What Worked Well

1. **Advisory Architectural Gates (`claude:opus:med`)**:
   - Instead of leaping into ad-hoc hacks, consulting Opus produced an elegant multi-milestone specification (M0–M5) comparing candidate injection mechanisms.
   - Identifying that HTTP proxy rewriting was unusable for starting idle turns—and instead repurposing `agymeter` strictly as an infallible read-only turn-boundary observer—prevented massive wasted effort.
2. **Lean-Sprint Milestone Cadence**:
   - Strict separation of read-only planning turns followed by write-authority resumption kept developer subagents (`luna:low` and `haiku:med`) disciplined, token-efficient, and prevented runaway edits.
   - Bounded milestones (M0 canary, M1 observer & turn state, M2 inbox queue, M3 supervisor integration, M4 CLI dispatch) allowed rapid iterative delivery and incremental verification.
3. **In-Process PTY Testing**:
   - `internal/subagent/interactive_test.go` and `agyinbox_test.go` utilized real in-memory PTY pairs (`github.com/creack/pty`) to verify raw byte sequencing, bracketed paste framing, busy suppression, and FIFO persistence without relying on external system state.

---

## 3. Honest Post-Mortem (Failures, Bugs & Near-Misses)

### 3.1 The Missing Environment Variable Trap (Ticket 757)
- **Failure**: In the initial attempt at ticket 757, developer agent `dev-757` assumed `CODEX_THREAD_ID` or `CODEX_SESSION_ID` was exported to child shells. The unit tests passed because the test suite artificially set `t.Setenv("CODEX_THREAD_ID", "...")`.
- **Reality**: In real Codex sessions, the interactive CLI exports neither variable to child commands. As a result, the live session was still missing when inspected by the user.
- **Resolution**: Prompting `claude:haiku:med` exposed the flaw. The ticket was reopened and reimplemented using `subagent.DiscoverActiveCodexSessions`, which inspects `/proc` process trees and rollout state files.

### 3.2 The Raw Terminal Newline Collision (`\n` vs. `\r`)
- **Failure**: During the first live test of socket delivery to `my-agy`, the message appeared in the TUI but merely broke the line (`\n`) without submitting the prompt.
- **Root Cause**: In raw terminal mode, TUI input frameworks (like Bubbletea in AGY) interpret line feed (`0x0A` / `\n`) as a multiline soft break, requiring carriage return (`0x0D` / `\r`) to trigger submission.
- **Resolution**: Verified that M2's bracketed paste injection properly appends `\r` (`\x1b[200~<text>\x1b[201~\r`), and verified live that `\r` submitted the turn cleanly.

### 3.3 Session Struct Field Divergence (`ProcessPID` vs. `ProviderPID`)
- **Failure**: In Milestone M4, running `harnez agent send --name loom-agy` failed with:
  `Error: AGY session "loom-agy" is not supervised; use harnez-agy --meter for live delivery`.
- **Root Cause**: The supervision liveness check in `cmd/harnez/agent_send.go` checked `session.ProviderPID <= 0`. However, interactive and supervised sessions in `cmd/harnez/agent_interactive.go` and `cmd/harnez/agy_meter.go` record the running child process ID in `session.ProcessPID`!
- **Resolution**: Dispatched `dev-758-pidfix` to check `ProcessPID` (with fallback to `ProviderPID`) and ensure `sendAgyMessage` connects to `session.ControlSocket` rather than treating the session ID string as a socket path. Immediately fixed and verified live.

---

## 4. Quality & Invariants Audit

| Invariant / Quality Check | Status | Evidence & Notes |
|---|---|---|
| **Zero Zombie Guarantee** | PASSED | All developer subagents (`dev-754`, `dev-756`, `dev-757`, `dev-758-m0..m4`, `dev-758-pidfix`) were rated and deleted cleanly; `harnez agent list` shows zero dangling sprint workers. |
| **Strict Zero-Coding Host** | PASSED | Host Orchestrator strictly maintained delegation boundaries; all Go source code edits and tests were implemented by dispatched leaf workers. |
| **Quota-1 Compliance** | PASSED | Full repo test target `make test-q1` ran cleanly across all commits; formatting (`gofmt`), linting (`go vet`), and package tests remained 100% green. |
| **Backward Compatibility** | PASSED | Standalone `agy` and non-supervised sessions receive clean explanatory error messages; Claude channel messaging and Codex thread queueing remained completely intact. |
| **Live Integration Verification** | PASSED | Real-world validation against live running session `loom-agy`; message submitted, model processed, and returned answer `42`. |

---

## 5. Efficiency & Velocity Assessment

- **Total Milestones Delivered**: 8 discrete milestones/fixes across tickets 754, 756, 757, and 758.
- **Model Economy**:
  - `luna:low`: Fast, inexpensive workhorse for planning, M0/M1/M2/M3/M4 implementation, and targeted bug fixes.
  - `haiku:med`: Excellent diagnostic auditor for tracking down the missing `CODEX_THREAD_ID` root cause and implementing process discovery.
  - `opus:med`: High-leverage systems architect for drafting the supervisor queue blueprint in ticket 758.
- **Turnaround Speed**: Each lean-sprint milestone completed within 2 to 4 minutes end-to-end, including full repository compilation and `make test-q1` test suites.

---

## 6. Key Learnings & Evergreen Upstream

1. **Beware Environment Variable Assumptions in Agent Runtimes**:
   - Never assume that an interactive CLI tool exports its internal session identifiers or thread UUIDs to child processes or subshells. Process-table and state-file discovery must always be the primary detection route.
2. **Terminal Injection Must Respect TUI Raw Modes**:
   - Raw mode PTY injection must always submit with `\r` (CR) rather than `\n` (LF), and multi-line inputs must always be framed within bracketed paste markers (`\x1b[200~` and `\x1b[201~`) to avoid line-by-line truncation or premature execution.
3. **Typing Guard Is Mandatory for Interactive Injection**:
   - An agent or background system typing into an interactive user terminal without a keystroke cooldown will inevitably collide with active user input. The 3-second typing guard window proved completely effective in preventing prompt mangling.

---

## 7. File & Diff Summary

### Key Files Modified & Created
- `cmd/harnez/agent_interactive.go`: Wired `OpenInbox` and `TurnState` to `InteractiveOptions`.
- `cmd/harnez/agy_meter.go`: Added terminal-gated session registration and PTY supervision to `harnez-agy --meter`.
- `cmd/harnez/hook.go`: Wired non-blocking `tool-start` / `tool-end` events to the session control socket.
- `cmd/harnez/agent_send.go`: Added supervised AGY target resolution, sender framing, and `enqueue` socket dispatch.
- `cmd/harnez/channel.go`: Added supervised AGY discovery to `harnez channel list` and fixed active Codex process matching.
- `internal/agymeter/meter.go`: Added `Observer` interface and `functionCall` SSE response scanning.
- `internal/subagent/agyturn.go` (new): Mutex-protected `TurnState` tracking request, tool, and typing windows.
- `internal/subagent/agyinbox.go` (new): Durable FIFO queue with atomic file persistence.
- `internal/subagent/interactive.go`: Added control socket `enqueue`/`inbox`/`turn` actions and bracketed-paste deliver loop.
- `docs/AgentMessaging.md`: Updated with comprehensive architectural documentation of the supervised AGY delivery route.
- `docs/studies/AgentSendCommandStatus.md`: Reconciled provider status for Claude, Codex, and AGY.
