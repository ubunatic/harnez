# Orchestrated Agent Flow — Roles, Review Checklist, Pitfalls

How a host runs a sprint through `harnez agent` with one orchestrator and one leaf developer
per ticket. Read it before starting an orchestrated run, when an agent misuses
`harnez agent`, or when reviewing an orchestrator's work. Evidence and history:
`docs/feedback/2026-09-22-orchestrated-sprint-flow-report.md`. CLI reference:
`docs/HarnezAgentArchitecture.md`.

## 1. Shape

| Role | Model used | Started by | May start agents |
|------|------------|------------|------------------|
| host (human or interactive agent) | - | user | any role |
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

### Interactive host identity

An interactive Codex host may inherit another worker's `HARNEZ_AGENT_ROLE`,
`HARNEZ_SESSION_ID` and model through a long-lived app server. The environment
alone does not establish the current caller's identity. Harnez verifies the exact
thread's rollout metadata, uses its thread ID for host tracking, and retains leaf
restrictions when a live launcher is in the process ancestry or a legacy worker
record matches the provider thread. See
[architecture §2.10](HarnezAgentArchitecture.md#210-roles-and-delegation-depth)
and [issue 953](../issues/953-interactive-codex-sessions-are-treated-as-harnez-leaf-agents.md).

Verify both sides through the installed binary without role overrides: the host
must complete self-test progression **and launch an actual reviewer**; the spawned
leaf must still receive a refusal for agent management. A passing health check
alone does not prove dispatch works. Keep the reviewer name, provider thread,
parent identity and exit result in the ticket. Regression cases live in
`cmd/harnez/agent_caller_test.go`.

Host sandbox permissions remain a separate boundary: Harnez cannot make a
read-only session directory writable. Reconfigure the managed host and restart
it when necessary; a writable Go cache only addresses build-cache access.
Issue-closure attribution still defaults to inherited environment values;
[issue 965](../issues/965-issue-closure-records-stale-inherited-worker-model-and-session-in-interactive-codex-hosts.md)
tracks that remaining metadata problem. Supply verified `--model`/`--session`
values when known; do not guess a model from stale worker variables.

## 2. Loop per ticket

1. Preflight: confirm the ticket's premise on HEAD (grep, `harnez read`, `git log -S`); if
   obsolete, close it with the finding and dispatch nobody.
2. `harnez agent start --name dev-N --role developer --model luna --stream stats "<short handoff>"`
   pointing at the ticket file; add `--plan yes` for code so the plan is reviewed first, then
   `harnez agent resume --name dev-N --stream stats "go ahead"`.
3. Review the commit against the ticket's acceptance points and against the current code.
4. Follow-ups: at most two precise `resume` turns per ticket, then stop and escalate.
5. `harnez issues close N "<written outcome>"` (several tickets: `harnez issues close N M "..."`),
   the close refuses unless the dev's commit names the ticket (`--no-code` only for a ticket that needed no code);
   with `--amend-reason` only to correct an existing closure; repeated closes otherwise do nothing,
   then `harnez agent rate --name dev-N <1-5> "reason"` and `harnez agent delete --name dev-N`.
   Delete refuses unrated sessions without `--force`; `delete --all` skips them (ticket 526).

Start/resume stays attached until the helper's turn is done. Interactive hosts run
it as a tracked session background job with `HTO=0`, then consume its completion
signal (Codex: `write_stdin` until exit); never repeat a start or poll status/logs.
Ask for replies of at most 10 lines with exact test results.

## 3. Review checklist

The orchestrator's review matched the diff to the ticket text but missed the state of the
code and the environment in all three tickets. Check:

- The premise still holds on HEAD, and the change does not recreate what the ticket wanted
  gone (ticket 135).
- Copyable docs (`docs/practices`, `docs/lang`, `docs/other`) gain no project ticket links
  or project-specific names (ticket 156).
- A direct passing `go test` of the touched packages is stated. A Quota-1 "blocked, no files
  changed" is neither pass nor fail. "Pre-existing failure" needs proof: run just those
  tests in a detached worktree at the commit before the ticket (`git worktree add --detach`), as in
  ticket 726, which surfaced 729.
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
| Worker never commits (570, 569) | It fixes after its one red run, so its final tree is untested | host runs the one q1 on that tree and commits if green |
| Shared checkout blocks q1 (570 follow-up, 572) | Another session's uncommitted, unformatted files stop `make test-q1` at gofmt, before any test runs | commit only your own paths, report them as untested, rerun q1 once the tree is clean; do not `make install` over foreign WIP |
| Analyst finds no data | stale empty `~/.local/share/harnez/telemetry.db` | real DB is `~/.harnez/tool_catalog.sqlite` |
| Agent report seems cut off | the host piped it through `cut -c1-N` | never truncate agent output when reading it |
| Workers report the host's session id | they inherit `CLAUDE_CODE_SESSION_ID` and similar variables | parent lineage uses `HARNEZ_SESSION_ID`; telemetry gap in ticket 487 |
| `harnez init` rewrites unrelated `docs/*.md` copies | root copies drift behind their sources | `git checkout --` the unrelated ones |
| `agy -p "…"` grabs the wrong token as its prompt | `-p` consumes the next argv token as its value | put `-p "<prompt>"` last, after every other flag |
| `agy` writes output files into `~/.gemini/antigravity-cli/scratch/` instead of the working dir | agy defaults to its own scratch dir unless told otherwise | pass both `cmd.Dir = <dir>` and `--add-dir <dir>` |
| `agy --model claude-sonnet-4-6 --effort low` fails | agy rejects `--effort` for `claude-*` models, only for `gemini-*` | mark the model `effort: false` in `spec/agent.yaml` and omit the flag |
| `agy` exits 1 with no useful error | it still prints a JSON `{"status":"ERROR","error":"…"}` object on stdout even on a non-zero exit | parse stdout on failure before falling back to stderr (see also `claude`'s opaque `exit status 1`, ticket 498) |
| Commit message says `close 606, <outcome>` | the handoff prompt contained a literal placeholder and the worker copied it | write the real close reason into the prompt, or let the host close the ticket |
| Closed ticket still listed as open in `issues/README.md` | worker edited the ticket status by hand | always close via `harnez issues close`; run `harnez index` if stale |
| 18 tests fail in `internal/claude` and `cmd` without anyone touching them | another session's uncommitted `config.yaml` setting changes defaults the tests assume | name the foreign files and known failures in every handoff; never stage them |
| Worker "fixes" leave ticket scope half done (323, 610) | handoff listed goals but not every file or edge case | list the files to touch and the negative cases (ambiguous input, hex-like words) as acceptance points |
| User rejects the delivered design after M2 (726: skill read its spec as a runtime file) | the host approved a plan choice that changes how the deliverable works for the user | do not pause the sprint for the user: the host decides and lists such choices in the delivery summary for review; a user redirect becomes the next milestone's pre-work in the ticket (`harnez agent stop` the worker first if it is mid-turn on the rejected design) |
| `harnez agent delete` refuses with "unrated session" | sessions must be rated before deletion | `harnez agent rate --name <s> <1-5> "<reason>"`, then delete |
| Provider turn fails with an opaque CLI exit (595, 745) | the useful reason is in Codex JSONL turn errors or Claude's final result line | Harnez surfaces the provider reason when present and retains the CLI error as fallback; retry transient capacity failures and resume after a reported limit reset |
| Worker reports a hook "measured at µs" (735) | it timed the Go function in-process, not the hook process the agent starts | ask for the end-to-end time of the installed hook command, or check the row appears live |
| Parity or count claims in a worker report look clean but are not (002 M2) | the worker summarized only the matching parts | host reruns the check itself and reads the DIFF lines |
| Live-testing a batch of agy models felt slow and left stray state | one `--conversation` resume took 118s; leftover named sessions persist after the run | use empty scratch dirs with no `AGENTS.md`, one tiny prompt plus one resume per model, unique session names, `harnez agent delete` afterward (ticket 500) |
| Backgrounded `harnez agent resume … &` dies after 60s, its codex child keeps running untracked | the shell's `&` job sits inside `harnez exec`'s 60s default timeout | start agents with the harness's own background mechanism (e.g. Claude Code `run_in_background`), never `&`; to track an orphan, wait on its PID in a background loop |
| `agent start -d X` worker asks "which repository?" (800) | codex driver ignored `-d` for the working directory | fixed in 800; until every provider is verified, also `cd X` before starting |
| Two writers allocate the same ticket number (local 805 vs cloud 805) | each reads `issues/` from its own snapshot; a cloud agent works from GitHub, not the local tree | cloud agents use `harnez issues new --remote "Title"` to allocate permanent GH IDs using authenticated gh on the github remote |
| Cloud agent's branch no longer fast-forwards | the host pushed local workers' commits to main meanwhile | cherry-pick the branch's own commits (check `git diff --name-only <merge-base>`) and ask the agent to reset its branch to main |
| Two agents fixed the same ticket (loom 329) | a remote agent picked a ticket the host had already dispatched locally | remote agents name the ticket before starting; the host keeps one owner per ticket and merges the better fix plus the other's regression test |
| Worker states a false fact as a blocker (voxi: "`go 1.26.5` breaks `go install`") | unverified model claim | reproduce the claim before accepting the change; revert with the evidence in the commit message |
| Worker cannot run podman ("namespace cloning denied") | the quota-1 sandbox blocks rootless podman | the host runs `make container-test` itself before closing the CI ticket |
| `harnez agent send` cannot reach a running worker | send supports live Claude/Codex/AGY TUI sessions, not exec-mode turns | queue the follow-up as the worker's next round prompt |
| `gh run view --log-failed` returns HTTP 403 | the local `gh` token lacks admin rights for log download | CI writes failures as annotations (shown by `gh run view`); or use the cloud agent's GitHub MCP `get_job_logs` |
| Worker closes a ticket with green tests, host live check fails (2026-10-20: 815, 818, 820, 826) | unit fixtures miss real state: completed batch sessions, legacy global blocks, unreadable `/proc/1/ns/user`, goreleaser subdir binaries | host runs the real command (`harnez diff` before `apply`, real `dist/`, live send/resume) before pushing; reopen with M2 pre-work in the same ticket |

## 5. Finding out what agents ran

- Authoritative: Codex rollout logs, `~/.codex/sessions/YYYY/MM/DD/rollout-*<thread id>.jsonl`
  (every tool call with its command text). Identify a thread by the role text in its first
  prompt. Count `harnez agent` calls per thread to prove the delegation depth.
- Telemetry `~/.harnez/tool_catalog.sqlite` (`tool_calls`, `cli_invocations`, `harnez stats`)
  has per-thread `tool_calls`, but no role or parent column and no nested `harnez`
  subcommands of workers (ticket 487).
- A read-only advisor (`--role advisor`, `luna:med`) can do the extraction; give it exact
  sources and forbid raw dumps, then verify its key claim against the rollouts yourself.

## 6. Multi-repo PM sprint with a cloud agent (2026-10-17)

Shape used for harnez, loom, cati and voxi in one session:

- **Host = product manager.** Decides scope per repo, starts one `harnez agent` developer per
  repo (parallel across repos, never two writers in one repo), reviews each result, reruns
  the tests and `make container-test` itself, then pushes to both remotes (Codeberg origin,
  GitHub) so `main` stays equal.
- **Rounds, not marathons.** Each developer gets a 3-4 item task file; the next round is a
  `resume` with the review findings, so the worker keeps its context. Queue follow-ups in a
  round file while the worker runs.
- **Cloud agents allocate remote IDs.** Remote/cloud workers use
  `harnez issues new --remote "Title"` themselves; `CLAUDE_CODE_REMOTE` also selects this
  mode automatically. Authenticated `gh` creates an issue on the repo's `github` remote,
  allocating permanent `GH<n>` files and headers without host reservations or local counters.
  Missing gh/authentication is an error; never fall back to local numbers. GH IDs stay
  permanent through merges. Reconcile duplicate GH files preserving their ID; only local
  numeric duplicates use `harnez issues mv -d . <file>`.
- **Cloud agent = GitHub resource.** A Claude cloud session with GitHub MCP reads job logs,
  watches CI (saving the host's `gh` rate limit), and pushes fixes to a `ccr-*` branch that
  the host reviews and fast-forwards or cherry-picks. It owns `.github/` only, names a ticket
  before starting, and allocates GH ticket IDs with `issues new --remote`. Messages to it are one-way from
  the host; its reports arrive as cross-session messages.
- **Quota routing.** Check `harnez usage` before each round. Heavy refactors on `opus`, routine
  work on `codex:sol`, coverage and lint sweeps on `agy:flash38`. When the Claude window runs
  low, Claude workers die with `claude resume: exit status 1` mid-turn (see §4): commit or
  park their WIP.

What paid off: read-only container mounts exposed tests that rewrote tracked fixtures; the
GitHub race job on 2 CPUs exposed unscaled timing budgets one per run until a sweep fixed all
of them (804); annotations made failures readable without log access.
