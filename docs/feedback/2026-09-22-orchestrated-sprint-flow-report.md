# Report: Orchestrated Sprint Flow with Enforced Roles (2026-09-22)

Goal: a `luna:med` orchestrator (started through `harnez agent`) runs the lean-sprint
workflow and lets a `luna:low` developer implement harnez tickets. Roles are fixed, no
agent starts agents that start agents, and the run stops after a stable flow with 2-3
finished tickets or when a fail-stop condition hits.

## Roles

| Role | Model | Started by | May start agents |
|------|-------|------------|------------------|
| host (Claude) | - | user | any role, unrestricted |
| orchestrator | `codex:luna:med` | host, `--role orchestrator` | developer, reviewer, advisor |
| developer | `codex:luna:low` | orchestrator, `--role developer` | nobody (leaf) |

The host wrote no ticket code during the flow. It reviews diffs, runs tests and commits
only what the orchestrator reports as reviewed, and writes this report.

## Fail-stop conditions

- The flow loops: the same milestone or ticket is retried three times without new
  progress.
- Agents keep calling `harnez agent` wrongly: three consecutive rejected calls by the same
  agent for the same reason.
- An agent starts a nested agent that starts agents (any refusal from the leaf guard that
  repeats after the agent was told).

## Milestone log

### M0 - Enforcement and guidance (done)

Nothing stopped a developer or an orchestrator from starting further agents, and workers
had no lineage. Delivered in ticket 486:

- `spec/agent.yaml` defines roles, their `spawns` list and their rules text; `--role`
  selects the role (default `developer`, a leaf).
- harnez exports `HARNEZ_AGENT_ROLE` and `HARNEZ_SESSION_ID` (the parent id) to the worker.
  A test caught that the first version recorded the child as its own parent.
- Leaf roles are refused by every mutating verb, root form and slash command;
  `list`, `status`, `models` stay available. An orchestrator cannot start an orchestrator.
- The role rules are added to the preamble of every turn, so subagents get the guidance
  without reading a doc. Human-facing text is one short section in
  `docs/practices/AgenticLoop.md` plus `--role` in the sprint commands.
- The lean-sprint skill is read from `docs/commands/lean-sprint.md` in the repo. The
  installed Codex copy predates these changes and refreshing it needs `harnez apply`,
  which changes global config, so it was not run.

Tickets planned for the flow: 156 (documentation), 268 (`harnez exec` timeout, code and
tests), and a third chosen after these two.
