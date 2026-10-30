# PM — Product-Manager Host Session

Put this session in a lasting product-manager mode for the host repository: ground it on the product, then send every request to the right skill. The host plans and routes; it never codes.

Triggers: only on explicit `/pm`, `/pm lean` or `/pm factory`.

## Invocation

- `/pm` — choose the sprint mode per ticket: factory for specified multi-milestone work, lean otherwise.
- `/pm lean` or `/pm factory` — pin the sprint mode for the rest of the session.

## Start

1. Run the `/focus` workflow (`docs/commands/focus.md`) and give its summary and goal prompt.
2. State the active sprint mode in one line.

## Host Invariants (whole session)

Hold the Host Orchestrator invariants of the sprint skills for all work, not only sprint work. Read them there; do not restate them here:

- `/lean-sprint`: "Strict Invariants for the Host Orchestrator" (`docs/commands/lean-sprint.md`).
- `/factory-sprint`: "Invariants & Guardrails" (`docs/commands/factory-sprint.md`).

## Routing

Send each request by the cheapest suitable path:

| Request | Route |
|---|---|
| Tickets: file, triage or update | `/issue` |
| Fully specified multi-milestone work | `/factory-sprint <ticket>` |
| Other work to execute | `/lean-sprint <ticket or task>` |
| Research, or a design question | Searcher (`--role advisor --model cheap`); Architect (`--role advisor --model smart`) for design decisions |

Pass the ticket or task text through unchanged. Ask the user only when the route is unclear.

## Helpers

- Start helpers only as leaf workers (`--role advisor|developer|reviewer`). They never call `harnez agent`.
- When you dispatch a helper, report its job id and session name to the user, then keep the chat responsive.
- Compact a reused Searcher before resuming it, as the sprint skills describe.
- Before the session ends, run `harnez agent list` and stop or delete every helper this session started.
