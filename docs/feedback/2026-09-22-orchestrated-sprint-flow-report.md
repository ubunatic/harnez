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

### M1 - Ticket 156, documentation (done, 2 orchestrator turns)

Orchestrator `sprint-orch` (`luna:med`, role orchestrator) started `dev-156` (`luna:low`,
role developer) with one `harnez agent start`, reviewed the commit, closed the ticket and
deleted the developer: 1m57s, no retries, no nested agent calls, roles as designed.
`harnez agent list` afterwards showed only the orchestrator.

The host review found what the orchestrator's review missed, and sent it back as a second
turn (new developer `dev-156b`, 1m31s, same clean pattern):

| Finding | Cause | Fix |
|---------|-------|-----|
| A copyable doc linked a project ticket and used a project-specific name | the review checked the diff against the ticket only | orchestrator prompt now names this check; `dev-156b` rewrote the section to 7 lines |
| Both workers reported `make test-q1` failing as "pre-existing" | (a) about 20 agent tests were not isolated from `HARNEZ_AGENT_ROLE`, a bug of the role feature; (b) Quota-1 blocks a repeat test run when no source changed, which reads like a failure | (a) fixed in `a04a1ef` with a `TestMain`; (b) prompts now say a Quota-1 block is neither pass nor fail |

Reported "two exec-hook assertion failures" could not be reproduced: the suite passes
directly, through `harnez exec`, with Codex worker environment variables, and with four
runs in parallel. Treated as unproven, not as a known flaky test.

Probe with a throwaway developer confirmed the lineage: inside the worker
`HARNEZ_AGENT_ROLE=developer` and `HARNEZ_SESSION_ID` is the worker's own session name,
which becomes the parent id of anything it starts.

Flow stability so far: 3 `harnez agent` calls by the orchestrator, 0 rejected, 0 retries.

### M2 - Ticket 268, `harnez exec` timeout (done, 2 orchestrator rounds)

Code plus tests. The orchestrator used the plan gate (`--plan yes`), reviewed the plan and
the commit, and rejected the first result. Total: 3 orchestrator turns for the two
tickets so far, 5 developer sessions, no host code.

What went wrong, and why it matters for the flow:

- The ticket was implemented as specified, and its 60s default then **killed the flow
  itself**: `harnez exec` wraps every agent shell command, so the developer's `go test` and
  its agent turn ran into the new deadline (exit 137, `timeout kill after 1m0s`), leaving
  partial uncommitted edits. The orchestrator refused to accept them, correctly.
- Host action: `exec.timeout: 30m` in the repo `config.yaml` (commit `bad6b76`), using the
  feature's own override. Round 2 added the missing pieces the orchestrator's review
  asked for: a process-group kill test, tests that never wait for the real 60s, and an
  exemption of `harnez agent` (blocking by design) from the implicit default, also for the
  ⚙ alias. Verified by the host: `go vet` and the full suite are clean.

### Analytics: what the agents actually ran

Read-only advisor `analyst` (`luna:med`, role advisor) plus the host's own check of the raw
Codex rollout logs. The rollouts are the authoritative source.

| Thread | Role | Tool calls | `harnez agent` calls | Native subagent spawns |
|--------|------|-----------:|---------------------:|-----------------------:|
| `sprint-orch` | orchestrator | 36 | 12 | 0 |
| six developer helpers | developer | 5-10 each | 0 | 0 |
| `analyst` | advisor | 21 | 0 | 0 |

- **Delegation depth held:** exactly one level. Only the orchestrator called `harnez agent`
  (5 starts, 3 resumes, deletes, all with `--role developer --model luna --stream stats`;
  `--plan yes` on the code ticket). No leaf-guard refusal was ever triggered, and there was
  no native subagent use.
- **Zero-coding held:** the orchestrator ran only `harnez agent`, `git log/diff`, `sed -n`,
  `harnez issues done` and `harnez rate`; no `apply_patch`, no `sed -i`, no commit.
  Developers used shell reads (`sed -n`, `rg`), `git diff/add/commit`, `gofmt`, `go`, `make`
  and `apply_patch`.
- **Interaction friction:** one malformed call (`exec: no command given`) that the
  orchestrator fixed itself; timeout kills from the 60s default (fixed above); Quota-1 blocks
  read as test failures by two workers (now covered in the prompts).
- **Two analyst errors, both traced to the host:** it was given a wrong DB path (an empty
  stale file) and its first report was cut by the host's own `cut`. Its claim that telemetry
  session ids do not map to Codex thread ids was wrong: `tool_calls` uses the thread id.
- **Telemetry gap (ticket 487):** no role or parent column, and nested `harnez` subcommands
  of workers are not recorded, so "which commands did agent X run" needs the rollouts.

Flow stability so far: 2 tickets closed, 0 rejected `harnez agent` calls, 0 loops, 1 systemic
hazard found and fixed (the timeout default).
