# Harnez Server Architecture (proposed)

**Status**: Proposal (2026-10-20), nothing below the existing drivers is built yet.
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

## Roadmap

1. Extract start/resume/compact orchestration from Cobra handlers into a service; document each
   provider's guarantees. No behavior change.
2. Transactional ledger (SQLite/WAL, versioned schema), idempotent submit, import existing sessions.
3. `harnez serve` supervisor: leases, restart reconciliation, quota wake-ups, crash quarantine.
   Canary: kill client, worker and server separately.
4. Immediate task handles, cursor subscriptions, outbox delivery; move CLI, MCP and TUI onto it.
5. Unattended goals with enforced limits, durable approvals, cancellation trees.
6. ACP, web, Slack and cloud adapters with authenticated sessions; cross-provider handoffs;
   optional Pi Durable worker bridge.

## Prior art

- Pi Durable (Earendil Works, experimental): storage-backed harness that saves each step before
  showing it; `requestId` dedup, replay-safe vs. interrupted tools, bounded watchers.
  <https://github.com/earendil-works/pi/blob/main/packages/durable/README.md>
- Pi Pocket: single Node server + one SQLite file, SSE clients, durable inbox, schedules,
  `/until` checks, Lancet Guard approvals saved across restart, push notifications. Not a sandbox.
  <https://tannermidd.github.io/pi-pocket/durable.html>,
  <https://tannermidd.github.io/pi-pocket/architecture.html>, <https://github.com/TannerMidd/pi-pocket>
- `disler/durable-pi`: per-device backend over Pi Durable. <https://github.com/disler/durable-pi>
- Comparables: Temporal (event-history replay), DBOS (database step checkpoints), Cloudflare
  Durable Objects (addressed actor + transactional storage).
