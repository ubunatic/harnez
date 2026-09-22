# Orchestrated Agent Flow — Roles, Review Checklist, Pitfalls

How a host runs a sprint through `harnez agent` with one orchestrator and one leaf developer
per ticket. Read it before starting an orchestrated run, when an agent misuses
`harnez agent`, or when reviewing an orchestrator's work. Evidence and history:
`docs/feedback/2026-09-22-orchestrated-sprint-flow-report.md`. CLI reference:
`docs/HarnezAgentArchitecture.md`.

## 1. Shape

| Role | Model used | Started by | May start agents |
|------|------------|------------|------------------|
| host (human or Claude) | - | user | any role |
| orchestrator | `codex:luna:med` | host, `--role orchestrator` | developer, reviewer, advisor |
| developer | `codex:luna:low` | orchestrator, `--role developer` | nobody (leaf) |
| reviewer, advisor | any | orchestrator or host | nobody (leaf, read-only) |

Delegation is one level deep, one developer at a time, one developer session per ticket
(deleted after use). The orchestrator never edits files; the ticket is the message channel.
The orchestrator follows `docs/commands/lean-sprint.md` (read from the repo: installed skill
copies only refresh with `harnez apply`).

Enforcement, not politeness: roles and their rules live in `spec/agent.yaml`. harnez stores
the role on the session, exports `HARNEZ_AGENT_ROLE` and `HARNEZ_SESSION_ID` (the worker's
session name, the parent id of anything it starts) to the provider process, adds the role
rules to every turn, and refuses `start`, `resume`, `stop`, `delete`, `compact`, `chat` and
root prompts for leaf callers. A new session without `--role` is a developer.

## 2. Loop per ticket

1. Preflight: confirm the ticket's premise on HEAD (grep, `harnez read`, `git log -S`); if
   obsolete, close it with the finding and dispatch nobody.
2. `harnez agent start --name dev-N --role developer --model luna --stream stats "<short handoff>"`
   pointing at the ticket file; add `--plan yes` for code so the plan is reviewed first, then
   `harnez agent resume --name dev-N --stream stats "go ahead"`.
3. Review the commit against the ticket's acceptance points and against the current code.
4. Follow-ups: at most two precise `resume` turns per ticket, then stop and escalate.
5. `harnez issues done N "..."`, `harnez agent delete --name dev-N`.

Every call blocks until the helper's turn is done; wait, never poll, never repeat a call.
Ask for replies of at most 10 lines with exact test results.

## 3. Review checklist

The orchestrator's review matched the diff to the ticket text but missed the state of the
code and the environment in all three tickets. Check:

- The premise still holds on HEAD, and the change does not recreate what the ticket wanted
  gone (ticket 135).
- Copyable docs (`docs/practices`, `docs/lang`, `docs/other`) gain no project ticket links
  or project-specific names (ticket 156).
- A direct passing `go test` of the touched packages is stated. A Quota-1 "blocked, no files
  changed" is neither pass nor fail. "Pre-existing failure" needs proof.
- Side effects on the running system: `make install` replaces the binary the other agents
  are executing (the `harnez exec` 60s default then killed the flow itself, ticket 268).
- The reviewer's own acceptance is honest: do not close a ticket with a known gap.

## 4. Pitfalls found

| Symptom | Cause | Fix |
|---------|-------|-----|
| Worker `go test` and agent turns die with exit 137 | `harnez exec` default timeout 60s applies to every wrapped shell command | `exec.timeout` in the repo `config.yaml`; `harnez agent` is exempt from the implicit default |
| About 20 agent tests fail only inside a worker | tests read `HARNEZ_AGENT_ROLE`/`HARNEZ_SESSION_ID` from the worker's environment | `TestMain` in `cmd/harnez` clears them; use `t.Setenv` |
| Two `internal/quota1` tests fail for everyone | a worker's Quota-1 command created a junk `/tmp/.git`, which the temp-dir tests then resolve into | remove it; ticket 488 |
| `make test-q1` "fails" in a worker | Quota-1 blocks a repeat run when no source changed | tell workers it is neither pass nor fail |
| Analyst finds no data | stale empty `~/.local/share/harnez/telemetry.db` | real DB is `~/.harnez/tool_catalog.sqlite` |
| Agent report seems cut off | the host piped it through `cut -c1-N` | never truncate agent output when reading it |
| Workers report the host's session id | they inherit `CLAUDE_CODE_SESSION_ID` and similar variables | parent lineage uses `HARNEZ_SESSION_ID`; telemetry gap in ticket 487 |
| `harnez init` rewrites unrelated `docs/*.md` copies | root copies drift behind their sources | `git checkout --` the unrelated ones |
| `agy -p "…"` grabs the wrong token as its prompt | `-p` consumes the next argv token as its value | put `-p "<prompt>"` last, after every other flag |
| `agy` writes output files into `~/.gemini/antigravity-cli/scratch/` instead of the working dir | agy defaults to its own scratch dir unless told otherwise | pass both `cmd.Dir = <dir>` and `--add-dir <dir>` |
| `agy --model claude-sonnet-4-6 --effort low` fails | agy rejects `--effort` for `claude-*` models, only for `gemini-*` | mark the model `effort: false` in `spec/agent.yaml` and omit the flag |
| `agy` exits 1 with no useful error | it still prints a JSON `{"status":"ERROR","error":"…"}` object on stdout even on a non-zero exit | parse stdout on failure before falling back to stderr (see also `claude`'s opaque `exit status 1`, ticket 498) |
| Live-testing a batch of agy models felt slow and left stray state | one `--conversation` resume took 118s; leftover named sessions persist after the run | use empty scratch dirs with no `AGENTS.md`, one tiny prompt plus one resume per model, unique session names, `harnez agent delete` afterward (ticket 500) |

## 5. Finding out what agents ran

- Authoritative: Codex rollout logs, `~/.codex/sessions/YYYY/MM/DD/rollout-*<thread id>.jsonl`
  (every tool call with its command text). Identify a thread by the role text in its first
  prompt. Count `harnez agent` calls per thread to prove the delegation depth.
- Telemetry `~/.harnez/tool_catalog.sqlite` (`tool_calls`, `cli_invocations`, `harnez stats`)
  has per-thread `tool_calls`, but no role or parent column and no nested `harnez`
  subcommands of workers (ticket 487).
- A read-only advisor (`--role advisor`, `luna:med`) can do the extraction; give it exact
  sources and forbid raw dumps, then verify its key claim against the rollouts yourself.
