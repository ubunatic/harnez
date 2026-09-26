# 597 — Add ticket-watcher / peer-assistant agent skill for multi-session coordination

**Status**: Closed — M1 skill+packaging, M2 ticket watching delivered; host-reviewed, tests green
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Skills / Multi-Agent Coordination / Practices
**Related**: `docs/commands/lean-sprint.md`, `docs/commands/LeanSprinter.md`, `docs/commands/HarnezAdvisor.md`, `issues/581`

---

## Goal

`/goal`: Create a reusable assisting-agent skill (e.g. `/peer-assistant` or `/ticket-watcher`) that equips secondary/assisting agent sessions to monitor incoming issue tickets, receive cross-session peer messages from active host agents, autonomously triage/dispatch tasks to low-cost worker agents, and persist decisions and findings directly into tickets and durable `docs/`.

## 1. Problem & Motivation

In multi-session workflows (e.g. running 2–3 parallel host sessions alongside a main development host), assisting agents can provide massive leverage by offloading discovery, research, and ticket execution.

During multi-session operations (2026-09-25), assisting Claude sessions were successfully instructed to operate as autonomous peer assistants:
- They actively observed the issue tracker for new/updated tickets.
- They remained open for peer messages from other named sessions (using cross-session communication / `SendMessage`).
- When nudged by a peer agent or when encountering incoming tickets, they triaged, created/updated tickets, and dispatched bounded work to low-cost workers (`luna:med`).
- They adhered to the core principle of writing decisions and findings back to durable documentation (`docs/studies/`, `docs/`) and tickets rather than holding context in ephemeral chat.

Codifying this workflow into a dedicated skill makes multi-agent peer assistance discoverable, consistent, and immediately reusable.

## 2. Technical Specification & Skill Workflow

### Assisting Agent Role & Mindset
The assisting agent is an autonomous orchestrator operating in support of other active agent sessions:
- **Peer-Aware**: Open to direct messages/nudges from peer host sessions (e.g. discovering active sessions by name).
- **Ticket-Driven**: When nudged or when new tasks arise, file or claim a ticket, prioritize it, and execute it through the standard loop.
- **Durable Documentation First**: Always persist architectural decisions, evaluation notes, and findings into `docs/studies/` and tickets rather than accumulating ephemeral memory in chat.

### The Assisting Loop Pattern
When incoming tickets or peer requests arrive:
1. **Quick Assessment**: Rapidly triage scope, priority, and prerequisites from the ticket description or peer message.
2. **Low-Cost Research Dispatch**: Spawn a low-cost research worker (e.g. `luna:med`) to explore code, trace dependencies, or verify premises.
3. **Record Findings**: Write research results and implementation plans directly into the ticket.
4. **Low-Cost Development Dispatch**: Dispatch a low-cost developer worker (e.g. `luna:med`) to implement the changes.
5. **Review & Commit**: Conduct milestone review and verification following the `/lean-sprint` protocol.
6. **Web Search & Escalation (as needed)**: Dispatch search/probe helpers (e.g. `luna:med` search agent) if external documentation is required.
7. **Document Sync**: Record key insights, decisions, and outcomes into `docs/studies/` or relevant evergreen docs.

## 3. Implementation & Acceptance Criteria

- Create the new skill definition in `docs/commands/` (e.g. `docs/commands/PeerAssistant.md` or `docs/commands/TicketWatcher.md`) and package into managed skill templates.
- Define clear invocation syntax (e.g. `/peer-assistant`, `/ticket-watcher`).
- Include the 7-step loop and cross-session communication protocol.
- Document in `docs/AgenticLoop.md` / `docs/practices/AgenticLoop.md` under multi-session coordination practices.

## Milestones

- **M1 delivered (skill + packaging)** — `4c478a0`: `docs/commands/PeerAssistant.md`, `config.yaml` registration,
  install test for all skill targets, AgenticLoop multi-session section. `make test-q1` green (host run).

### M2 — Ticket watching (Pre-Work / Required Refinements)

- The ticket goal is a *ticket watcher*, but M1 only disclaims continuous monitoring and gives no way to watch.
  Add a short "Watching" section to `docs/commands/PeerAssistant.md`: how to check for new or updated tickets
  (`harnez find -d <repo> issues -a status:open`, plus `git log` on `issues/` since the last check), and how to
  repeat that check with the harness's own scheduling/monitor facility when one exists. If none exists, check at
  each turn. Keep it honest: it only claims watching when a scheduled check is actually running.
- Extend the test markers to cover the new section.
