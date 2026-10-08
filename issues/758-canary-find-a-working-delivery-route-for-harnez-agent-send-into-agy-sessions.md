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
- Architectural opportunity (Phase 2): **Supervised AGY execution.** When harnez is the parent
  process of `agy`, it owns the child's terminal and its network proxy, so it can queue messages
  and type them in at turn boundaries — the AGY analogue of `codex queue`.

### 2.1 Supervisor code as it exists today (2026-10-08)
- **PTY injection already exists for `harnez agent start -i`.** `runInteractivePTY`
  (`internal/subagent/interactive.go:143`) starts AGY on a PTY (`creack/pty`) inside
  `agymeter.RunWithEnvDirRunner`, and `serveControls` (`interactive.go:267`) listens on a per-session
  Unix socket `<storeDir>/<id>.sock` (`cmd/harnez/agent_interactive.go:97`). Action `prompt` writes
  `prompt+"\n"` straight into the PTY; `harnez agent resume` on an interactive session already uses
  it (`cmd/harnez/agent_run.go:766`). Gaps: it types immediately (no idle check, so it can land
  mid-turn or inside the user's half-typed input), a `\n` inside the message submits early, and
  there is no queue, message ID or delivery status.
- **`harnez-agy --meter` has no injection point.** The launcher (`HarnezAgyLauncherContent`,
  `internal/claude/apply.go:901`) execs `harnez agy-meter-run -- <agy> ...`
  (`cmd/harnez/agy_meter.go`), which calls `agymeter.RunWithEnv` → `runChild`
  (`internal/agymeter/meter.go:115`): the child inherits stdin/stdout directly, no PTY, no control
  socket, no session record. Without `--meter` the launcher execs `agy` directly (no supervisor).
- **The meter sees every model call.** `Meter.forward` (`meter.go:331`) MITMs
  `daily-cloudcode-pa.googleapis.com` and wraps each response body in `responseObserver`
  (`meter.go:469`), which already parses the stream (`parseResponse`, `meter.go:546`). It knows when
  a request starts and when its response stream ends, which is the raw material for a turn-state
  signal.
- **AGY hooks already call harnez per tool call** (`harnez hook agy` / `agy-post`,
  `cmd/harnez/hook.go`) and receive `conversationId`; the hook process inherits the child env, so it
  can see `HARNEZ_SESSION_ID` when the supervisor sets it.
- `agent send` (`cmd/harnez/agent_send.go:290`) and `channel list` (`cmd/harnez/channel.go:210`)
  know only Claude channel endpoints and Codex threads.

### 2.2 Candidate injection mechanisms
| | A: PTY typing | B: proxy request rewrite | C: turn-boundary mailbox |
|---|---|---|---|
| How | Write message bytes into the child's PTY master, as if typed | Append a user `contents` part to the next outbound model request in `Meter.forward` | Supervisor holds a FIFO inbox and delivers only when AGY is idle |
| Starts a turn when AGY is idle | Yes (Enter submits) | **No** — proxy only acts when AGY sends a request | Yes, via A |
| In AGY's own transcript / resumable | Yes, it is a real user turn | **No** — AGY's local history never contains it; the model sees it once, then it vanishes | Yes, via A |
| Depends on private API schema | No | Yes, undocumented request body | No (turn signal reads the stream, never rewrites it) |
| Risks | Lands mid-turn or into the user's half-typed line; `\n` submits early | Silent history divergence, breaks on schema change, alters traffic | Needs a reliable idle signal |

**Decision:** C as the delivery policy, A as the transport, and the meter (B's position) used
**read-only** as the idle signal. Request rewriting (B) is rejected as a delivery route.

### 2.3 Recommended design
```
harnez agent send --name agy-x "msg"
        │  (unix socket: {"action":"enqueue",...})
        ▼
supervisor (harnez agy-meter-run / agent start -i)
  ├─ Inbox: FIFO + <storeDir>/<id>.inbox/<msg-id>.json  (queued → delivered | failed)
  ├─ TurnState ◄── meter: request start / stream end / functionCall seen
  │            ◄── hook agy / agy-post: tool start / tool end (best effort)
  │            ◄── stdin copy: last user keystroke
  └─ deliver loop: when Idle() → bracketed paste + "\r" into PTY → mark delivered
        ▼
agy TUI on PTY ──HTTPS_PROXY──► Meter ──► daily-cloudcode-pa.googleapis.com
```
- **Supervised session = registered harnez session.** Both entry points (`agent start -i` and
  `harnez-agy --meter` on a TTY) run AGY through the same PTY supervisor, create a
  `subagent.Session` (provider `agy`, `ControlSocket` set, `Source` `"harnez-agy"` or the existing
  value) and remove the socket on exit. `agent send` and `channel list` discover AGY targets from
  this store; no new registry. `harnez-agy` sessions get an auto name such as
  `agy-<dir-basename>-<4 hex>`, overridable with `HARNEZ_AGY_NAME`.
- **Idle rule.** `Idle()` is true when: no model request in flight, the last finished response
  contained no `functionCall` part (the model ended its turn rather than calling a tool), no open
  tool call from hooks, at least `quiet_ms` since the last of these events, and at least
  `typing_guard_ms` since the last user keystroke. Mid-turn messages wait; nothing interrupts a turn.
  If the meter could not start, the inbox is disabled and `enqueue` returns "no idle signal".
- **Typing.** Messages go in as one bracketed paste (`ESC[200~ … ESC[201~`) followed by `\r`, so
  multi-line text stays one prompt. Fallback if M0 shows AGY ignores bracketed paste: replace
  newlines with spaces.
- **Message frame.** Reuse `channelSender` + reply hint, as `codexMessageFrame` does, so the AGY
  model knows who sent it and how to answer (`harnez agent send --id <sender> ...`).
- **Persistence.** Each message is a JSON file written by atomic rename; a restarted supervisor of
  the same session ID re-queues files still `queued`. `send` returns once the file is written and
  acknowledged; it does not wait for delivery (same contract as Codex "queued").
- **Explicit `agent resume` on an interactive AGY session** also goes through `enqueue`, so the two
  paths cannot type over each other.
- Out of scope: non-supervised AGY processes (plain `agy`, `harnez-agy` without `--meter`) and
  print-mode workers; `send` names the `agent resume` route for those instead.

### 2.4 Data structures and endpoints
```go
// internal/subagent/agyinbox.go (new)
type InboxMessage struct {
	ID          string    `json:"id"`           // random hex
	SenderID    string    `json:"sender_id,omitempty"`
	Text        string    `json:"text"`         // full framed text
	State       string    `json:"state"`        // queued | delivered | failed
	QueuedAt    time.Time `json:"queued_at"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}
type Inbox struct { dir string; mu sync.Mutex; queue []*InboxMessage; wake chan struct{} }
func OpenInbox(dir string) (*Inbox, error)       // loads queued files
func (*Inbox) Enqueue(sender, text string) (*InboxMessage, error)
func (*Inbox) Next() *InboxMessage
func (*Inbox) Mark(id, state, errText string) error

// internal/subagent/agyturn.go (new)
type TurnState struct { mu sync.Mutex; inflight, openTools int; lastFunctionCall bool; lastEvent, lastKey time.Time }
func (*TurnState) RequestStarted()
func (*TurnState) ResponseFinished(functionCall bool)
func (*TurnState) ToolStarted(); func (*TurnState) ToolFinished()
func (*TurnState) Keystroke()
func (*TurnState) Idle(now time.Time, quiet, typingGuard time.Duration) bool

// internal/agymeter/meter.go
type Observer interface { RequestStarted(); ResponseFinished(functionCall bool) }
// Meter.observer set via a new RunWithEnvDirRunnerObserved(..., obs Observer, ...);
// forward() calls RequestStarted; parseResponse reports functionCall at stream end.
```
Control socket (`<storeDir>/<id>.sock`, JSON line protocol, existing `controlRequest`):
| action | request fields | response | caller |
|---|---|---|---|
| `enqueue` | `prompt`, new `sender` | new `message_id`, `queued` (depth) | `agent send`, `agent resume` |
| `inbox` | — | new `messages []InboxMessage` | `agent status`, tests |
| `turn` | new `event`: `tool-start`/`tool-end` | — | `harnez hook agy` / `agy-post` |
| `prompt`, `compact`, `stop` | unchanged | unchanged | existing |

`SendControl` keeps its signature; add `SendControlRequest(ctx, socket, controlRequest) (controlResponse, error)`.

`agent send` output for AGY: `{"status":"queued","provider":"agy","session_id":"<harnez id>","message_id":"<id>"}`.

Spec (`spec/channel.yaml` + its JSON schema in `spec/schemas/`, per `docs/Spec.md`), new `agy:` block:
`quiet_ms` (start 1500), `typing_guard_ms` (3000), `max_message_bytes` (16384),
`paste_mode` (`bracketed` | `flatten`). No Go defaults.

## 3. Canary & Prototype Plan
Phase 1 (Done): Probed standalone AGY CLI capabilities, lifecycle hooks, and resume. Documented in `docs/studies/2026-10-08-agy-delivery-route-canary.md`.

Phase 2 (Current Focus): **Supervised queue injection.** Milestones in order; each ends with
`make test-q1`, `make install` and a commit.

**M0 — PTY canary (needs a real TTY; the Phase 1 sandbox had none).** Run inside
`harnez agent start -i` with an `agy` session on `gemini-3.6-flash-low`, driven through the existing
`prompt` control action and `HARNEZ_AGY_METER_DEBUG=1`. Record results as a "Phase 2" section in
`docs/studies/2026-10-08-agy-delivery-route-canary.md`:
- P1 idle typing: `prompt` while the TUI is idle → a new turn starts and the reply is visible.
- P2 mid-turn typing: `prompt` during a long tool call → note whether AGY queues it, drops it, or
  mangles the turn. (Decides whether waiting for idle is mandatory; the design assumes yes.)
- P3 bracketed paste: three-line message → arrives as one prompt or not (decides `paste_mode`).
- P4 turn signal: in the meter debug log, confirm the response stream exposes `functionCall` parts
  for tool steps and none on the final answer; measure the gap between a tool result and the next
  request.
- P5 output noise: does the idle TUI emit PTY output (cursor blink)? Confirms PTY output must not
  be used as the idle signal.
Stop and report if P1 fails: then no supervised route exists and the ticket closes as resume-only.

**M0 progress (2026-10-08):** This worker had no TTY (`test -t 0` was false), so the real AGY
interactive checks P1–P5 remain open. The in-process PTY canary verified that the control-socket
`prompt` action writes bytes into terminal input, appends `\n`, preserves embedded newlines as
separate terminal lines, and does not add bracketed-paste framing. This confirms the existing
transport but cannot establish AGY TUI behavior. The immediate, ungated action is unsuitable for
safe multiline/mid-turn delivery; retain idle-gated queueing and finish P1–P5 on a TTY-enabled host.
Details: [AGY delivery route canary](../docs/studies/2026-10-08-agy-delivery-route-canary.md).

**M1 — Turn signal in the meter.** `internal/agymeter/meter.go`: `Observer` interface,
`RunWithEnvDirRunnerObserved`, `forward` → `RequestStarted`, `parseResponse` → `ResponseFinished`
with a `functionCall` scan (field names from P4). `internal/subagent/agyturn.go` + tests for
`Idle()` with an injected clock. Meter tests: a fake upstream stream with and without a
`functionCall` part fires the observer correctly; metering output unchanged.

**M1 progress (2026-10-09):** Implemented the meter observer callbacks and SSE `functionCall`
detection, plus a mutex-protected `TurnState` with an injected clock and idle-window checks. Added
fake-upstream observer tests and idle-state tests. `make test-q1` and `make install` passed.

**M2 — Inbox and deliver loop.** `internal/subagent/agyinbox.go` + tests (enqueue, restart reload,
atomic files, `max_message_bytes`). `internal/subagent/interactive.go`: `serveControls` takes the
inbox and turn state; add `enqueue`, `inbox`, `turn` actions; wrap the stdin copy to call
`Keystroke()`; deliver goroutine waits on `wake` or a ticker, checks `Idle()`, writes the paste,
marks delivered. Tests with a fake PTY writer (`bytes.Buffer`) and a fake turn state: no write
while busy, FIFO order, paste framing.

**M3 — `harnez-agy --meter` becomes a supervisor.** `cmd/harnez/agy_meter.go`: when stdin is a
TTY, create a session record in the agent store (store dir and naming from
`cmd/harnez/agent_interactive.go`), set `HARNEZ_SESSION_ID` / `HARNEZ_AGY_METER_SESSION_ID`, and
run through a new exported `subagent.RunSupervisedAgy` that shares `runInteractivePTY`;
without a TTY keep today's `runChild` path. Mark the session ended and drop `ControlSocket` on
exit. `HarnezAgyLauncherContent` needs no change (it already execs `agy-meter-run`); confirm
`internal/claude/bash_shim_test.go` still passes. `cmd/harnez/hook.go`: `agy` / `agy-post` send a
best-effort `turn` event (≤100 ms timeout, errors ignored, hook output unchanged) when
`HARNEZ_SESSION_ID` resolves to a session with a live socket.

**M4 — `agent send` and `channel list`.** `cmd/harnez/agent_send.go`: load AGY sessions with a live
`ControlSocket` and live `ProviderPID`; add them to `resolveAgentSendTarget` candidates
(`provider=agy`) and `liveAgentNames`; `sendAgyMessage` frames the text like
`codexMessageFrame`, calls `enqueue`, prints the queued JSON; update `Short`/`Long`/`Example`.
`--wait` stays Claude-only (error for AGY). `cmd/harnez/channel.go`: `mergeChannelSessions` lists
supervised AGY sessions as live; `isLiveChannelSession` accepts `agy`. `cmd/harnez/agent_run.go:766`:
interactive AGY resume uses `enqueue` instead of `prompt`. Tests for target resolution, ambiguous
names, and the not-supervised error message.

**M5 — End-to-end proof and docs.** In a real terminal: start `harnez-agy --meter`, run
`harnez channel list` (session shown live), `harnez agent send --name <it> "reply ACK-758-E2E"`
during a running turn and while idle; confirm delivery after the turn ends and the reply in the
TUI; resume the conversation with `agy --conversation <id>` and confirm the message is in
history. Record in the study doc. Update `agent send --help`, the relevant `docs/` page for agent
send, and ticket 656 M4 status. Pre-commit review by a fresh reviewer subagent (larger change).

### Acceptance
- `harnez agent send --name <supervised-agy>` returns `queued` with a message ID and the message
  starts a new AGY turn at the next idle point, never mid-turn.
- Multi-line messages arrive as one prompt; the user's half-typed input is not corrupted.
- Queued messages survive a supervisor restart of the same session ID.
- Non-supervised AGY sessions get a clear error naming `harnez-agy --meter` or `agent resume`.
