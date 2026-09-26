---
name: peer-assistant
description: Coordinate peer requests and ticket work across agent sessions
disable-model-invocation: true
---

# Peer Assistant

Use `/peer-assistant` when another active host session asks for help or when
you are explicitly assigned to assist across sessions. Treat the current
conversation and the available native messaging/session tools as authoritative;
do not claim to monitor messages or tickets continuously when no such facility
is active.

## Assisting Loop

1. Assess the request or ticket for scope, priority, prerequisites, and the
   expected handoff. Ask for missing decisions that materially affect the work.
2. Dispatch bounded, read-only research to a low-cost advisor when useful.
   Keep dispatch sequential by default and follow the role/delegation rules in
   `AgenticLoop.md`.
3. Record useful findings, decisions, and an implementation plan in the ticket
   or relevant durable documentation.
4. Dispatch implementation to a low-cost developer when appropriate. Keep one
   writer per shared workspace and give the worker concrete acceptance checks.
5. Review the result and verification against the ticket, following
   `/lean-sprint` for focused work. Ensure the ticket records outcome and any
   remaining work.
6. Use a separate search/probe helper or web research when an external premise
   needs verification; record the resulting evidence durably.
7. Sync durable learnings to the relevant evergreen documentation or
   `docs/studies/` when they apply beyond the ticket.

## Watching

- Check open tickets with `harnez find -d <repo> issues -a status:open`.
- Record the current commit as a checkpoint; next time inspect
  `git log <checkpoint>..HEAD -- issues/` for ticket changes, then advance it.
- Repeat checks with the harness's scheduling or monitor facility when one is
  available and actually running. Otherwise check at each turn; claim active
  watching only while a scheduled check is running.

## Peer Communication

- Reply to direct peer messages through the available native session
  communication surface; identify the ticket or task in the response.
- Use tickets for durable handoffs and concise peer messages for immediate
  coordination. Do not treat chat as the only record of a decision or result.
- Track every helper session you start and stop or delete it when its work is
  complete. Do not take over or stop unrelated sessions.
- Keep the host session responsive and report progress, blockers, and results
  with named tickets or milestones.
