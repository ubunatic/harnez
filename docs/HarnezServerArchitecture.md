# Harnez Server Architecture (proposed)

**Status**: Proposal (2026-10-20). Built so far: `harnez server` with the usage collector as its first
service (see "Already in place"); the session service, ledger and supervisor below are not built.
**Related**: [HarnezAgentArchitecture.md](HarnezAgentArchitecture.md), [AgentMessaging.md](AgentMessaging.md), [PiAgent.md](PiAgent.md)

Goal: turn harnez into an "any client" <-> "AI server harness": any client talks to one
`harnez serve` process that runs, persists and resumes agent sessions across providers,
durably (crash-safe) and optionally unattended ("level 3: out of the loop").

## Diagram

```
                        CLIENTS (any)
 ┌──────────┬──────────┬───────────┬──────────┬──────────┬───────────┐
 │ harnez   │ harnez   │ Claude /  │ Editors  │ Web /    │ Slack /   │
 │ CLI      │ TUI      │ Codex /   │ (ACP)    │ phone    │ chat /    │
 │          │ (jobs)   │ agy (MCP) │          │ (SSE)    │ cloud     │
 └────┬─────┴────┬─────┴─────┬─────┴────┬─────┴────┬─────┴─────┬─────┘
      │          │           │          │          │           │
 ┌────▼──────────▼───────────▼──────────▼──────────▼───────────▼─────┐
 │  ADAPTERS   auth: user → session · capability discovery            │
 │  MCP · ACP · SSE · chat bridges → one versioned JSON-RPC API       │
 │  submit(idempotency key) · status · cancel · approve · subscribe   │
 └──────────────────────────────┬─────────────────────────────────────┘
                                │  Unix socket (later: TLS remote)
 ┌──────────────────────────────▼─────────────────────────────────────┐
 │                         harnez serve                               │
 │                                                                    │
 │  ┌──────────────────┐  ┌──────────────────┐  ┌─────────────────┐   │
 │  │ Session / task   │  │ Scheduler        │  │ Policy & goals  │   │
 │  │ service (moved   │  │ leases + fencing │  │ level-3 limits: │   │
 │  │ out of CLI cmds) │  │ timers, quota    │  │ budget, time,   │   │
 │  │ start · resume · │  │ wake-ups, retry, │  │ tools, network, │   │
 │  │ compact · send   │  │ crash quarantine │  │ acceptance chk  │   │
 │  └────────┬─────────┘  └────────┬─────────┘  │ saved approvals │   │
 │           │                     │            └────────┬────────┘   │
 │  ┌────────▼─────────────────────▼─────────────────────▼────────┐   │
 │  │  EXECUTION LEDGER   SQLite / WAL (one writer)                │   │
 │  │  sessions · branches · submissions · tasks · attempts       │   │
 │  │  ordered events (cursor) · checkpoints · approvals          │   │
 │  │  inbox · timers · notification outbox · requester identity  │   │
 │  │  ── task state + event + outbox saved in ONE transaction ── │   │
 │  └──────────────────────────────┬───────────────────────────────┘   │
 │                                 │                                  │
 │  ┌──────────────────────────────▼───────────────────────────────┐   │
 │  │  WORKERS (isolated, leased)                                  │   │
 │  │  recover at provider-turn boundaries                         │   │
 │  │  safe tools → replay · unsafe tools → "uncertain", check     │   │
 │  └──────┬───────────┬───────────┬───────────┬──────────────────┘   │
 └─────────┼───────────┼───────────┼───────────┼──────────────────────┘
           │           │           │           │
     ┌─────▼────┐┌─────▼────┐┌─────▼────┐┌─────▼──────────┐
     │ Claude   ││ Codex    ││ agy      ││ Pi / Pi Durable│
     │ driver   ││ driver   ││ driver   ││ worker bridge  │
     └─────┬────┘└─────┬────┘└─────┬────┘└─────┬──────────┘
           ▼           ▼           ▼           ▼
       provider's own transcript (kept as a reference, never copied)
```

## Request flow

1. **Submit**: a client sends work with an idempotency key; a retry is accepted only once.
   The ledger saves the submission before anything runs.
2. **Schedule**: the scheduler leases the task to one worker; fencing prevents a second writer.
3. **Run**: the worker drives the provider one turn at a time and records each attempt and event.
4. **Policy**: an action outside the goal's limits pauses for a saved approval. With no client
   connected it stays blocked.
5. **Report**: clients subscribe after a cursor; late or slow clients get a snapshot. Notifications
   leave through the outbox.
6. **Recover**: after a crash, unfinished tasks resume at the last finished provider turn.
   Interrupted unsafe steps are marked uncertain and checked, never replayed blindly.

## Design rules

- **Native CLI sessions are the execution unit.** Workers drive the real `claude -p`,
  `codex exec`, `agy` (and `pi`) sessions so subscriptions (Claude Pro/Max, ChatGPT, Google)
  keep paying; no silent API fallback for subscription providers. Pi Durable and Pi Pocket call
  provider APIs through `pi-ai` (API keys or `/login` subscription tokens), which puts Claude on
  API billing, so they are not the execution engine. Consequences: the smallest checkpoint is a
  finished provider turn, crash recovery resumes the native session by provider ID, and policy is
  enforced through each CLI's own controls (permission modes, hooks, sandbox flags). A Pi Durable
  worker is only for API-billed models (OpenAI API, local/lmcoder). See 776.
- Durability (crash recovery) and autonomy (no human approving steps) are separate axes.
- State crash guarantees at commit boundaries, including the streaming loss window.
- Durable is not exactly-once: external effects need idempotency keys or postcondition checks.
- Persist approval and policy decisions with the task, so recovery never asks twice.
- Carry the requester identity through child agents for approvals, quota, spend and notifications.
- Provider transcripts cannot move losslessly between providers; cross-provider continuation
  is an explicit handoff branch with summary, artifacts and provenance.
- "Level 3" is our own operational definition: a goal runs unattended within delegated
  authority (acceptance checks, budget/time/iteration limits, tool/network scope) and ends in
  verified completion or a durable blocked outcome. Enforce limits at worker and tool
  boundaries, not only in prompts. No public taxonomy fixes this level number.
- Endpoints, limits and policy defaults live in `spec/` with schemas. `apply` and `init` never
  start the server.

## Already in place

Provider drivers (`internal/subagent/driver.go`), session records and resume
(`session.go`, `resume.go`), compaction and quota handling, the stdio MCP server
(`internal/mcp/server.go`), `agent send` and channel receipts (`internal/channel/`), and the
TUI job panel (`internal/tui/jobs/`). Session JSON uses atomic rename; that prevents torn reads
but is not a transactional task log.

**First service built (2026-10-20, issues 829-831; transport converged in 837):** `harnez server`
(`internal/server`) runs the usage data collector as its first service and is the single writer of
`telemetry.sqlite`. It serves its HTTP routes over a Unix socket under `$XDG_RUNTIME_DIR/harnez/`
(falling back to the state directory), with directory mode 0700 and socket mode 0600. The socket
probe provides the singleton: stale sockets are replaced, foreign listeners are rejected, and a
newer semver or equal-version newer build identity shuts down the old process before taking over.
`/status` reports version, process ID, and the executable mtime captured when that process started.
Services register their own routes. `harnez usage` starts/probes the server; statusline and hooks
POST updates: usage readings are dropped when no server answers, telemetry records go to a bounded
spool the server drains (830). A boundary test fails on any writable `telemetry.sqlite` open outside
the server. On Linux, `harnez server install|status|stop|uninstall` manages the systemd user service.
The unit restarts on failure. When a newer build finds that the old server is the unit's `MainPID`,
it rewrites the unit's `ExecStart` to its own executable and runs `systemctl --user restart`
instead of taking the socket, so any install path (`go install`, `make install`, a release) keeps
the server under systemd. Otherwise a successful takeover exits the old server with status 0,
avoiding a systemd restart race; status reports when the socket owner differs from the unit's
`MainPID` and prints the restart command that returns ownership to systemd. Other hosts can keep the server
available with `harnez usage -w`.

The built command remains `harnez server` and these local routes remain HTTP over the Unix socket.
JSON-RPC remains the target for the later agent-session API (roadmap step 4), behind transport
adapters; it is not required for usage collection.

## Roadmap

1. Extract start/resume/compact orchestration from Cobra handlers into a service; document each
   provider's guarantees. No behavior change.
2. Transactional ledger (SQLite/WAL, versioned schema), idempotent submit, import existing sessions.
   Engine choice open: DBOS Go v1.7 on SQLite is a candidate only inside the single server owner
   (issue 841). Its canary recovered a killed workflow from the last completed step, but costs
   ~26 ms warm startup and +12.6 MB, and concurrent multi-process init hit `SQLITE_BUSY`.
   Plain SQLite stays for telemetry, bench, sessions, usage and feedback. See
   [2026-10-21 DBOS study](studies/2026-10-21-dbos-sqlite-storage-research.md).
3. `harnez serve` supervisor: leases, restart reconciliation, quota wake-ups, crash quarantine.
   Canary: kill client, worker and server separately.
4. Immediate task handles, cursor subscriptions, outbox delivery; move CLI, MCP and TUI onto it.
5. Unattended goals with enforced limits, durable approvals, cancellation trees.
6. ACP, web, Slack and cloud adapters with authenticated sessions; cross-provider handoffs;
   optional Pi Durable worker bridge.

## Lessons from Pi Pocket's code (2026-10-20)

Audited `TannerMidd/pi-pocket` at `88511086` (single Node process, one SQLite owner, Pi Durable 1.1.0).

- **Two event products**: Pocket's SSE (`src/server/http/events.ts`) is a live projection with
  bounded queues and long-poll fallback; reconnect means a fresh snapshot. It is not a durable
  cursor stream. Harnez should keep both, separately: a lossy live view (coalesced ~90 ms room,
  ~400 ms lists) and a durable cursor/outbox for notifications.
- **Commit listener contract**: one listener processes unordered commit batches in deterministic
  passes, never writes back reentrantly (follow-up writes deferred), and derived data such as
  authorship is repaired at startup.
- **Requester IDs**: `u:<user>:<key>` for human-authored, `p:<user>:<key>` for sent on a user's
  behalf; the newest attributable message decides spend, approvals, schedules and subagents.
- **Durable timers**: schedules are tasks; overdue work sends once after restart, deduplicated
  by request ID. Approval and goal-check answers are task memos.
- **Fork policy**: each record kind declares fork behavior (copy or empty); fork cleanup commits
  with fork creation.
- **Bounded unattended loops**: `/until` (max 5 checks), `/watch` PR follow-up (max 10 messages),
  signed webhooks, per-user spend limits; approval-gated calls still wait for a person.
- **Operator surface**: a cross-session "Running now" view with Stop, and push notifications for
  completion, approval needs and mentions (some approvals answerable from the notification).
- **Not a sandbox**: steering, plan mode, invites, worktrees and in-process extensions are access
  control, not isolation; the browser tool can bypass the guard. Harnez must enforce limits at the
  worker and tool boundary.
- **Recovery tests to copy**: restart during scheduled send, subagent reports across restart,
  approvals with attribution across restart, duplicate request IDs (`test/*.test.ts`). Harnez
  needs a recovery matrix per driver: before/after submit, provider turn, tool effect, approval.
- **Not validated by Pocket**: multi-worker leases and fencing, multi-provider continuation, power
  loss durability, exactly-once side effects.

## Prior art

- Pi Durable (Earendil Works, experimental): storage-backed harness that saves each step before
  showing it; `requestId` dedup, replay-safe vs. interrupted tools, bounded watchers.
  <https://github.com/earendil-works/pi/blob/main/packages/durable/README.md>
- Pi Pocket: single Node server + one SQLite file, SSE clients, durable inbox, schedules,
  `/until` checks, Lancet Guard approvals saved across restart, push notifications. Not a sandbox.
  <https://tannermidd.github.io/pi-pocket/durable.html>,
  <https://tannermidd.github.io/pi-pocket/architecture.html>, <https://tannermidd.github.io/pi-pocket/>,
  <https://github.com/TannerMidd/pi-pocket> (features: `docs/features.md`, limits: `SECURITY.md`)
- `disler/durable-pi`: per-device backend over Pi Durable. <https://github.com/disler/durable-pi>
- Comparables: Temporal (event-history replay), DBOS (database step checkpoints), Cloudflare
  Durable Objects (addressed actor + transactional storage).
