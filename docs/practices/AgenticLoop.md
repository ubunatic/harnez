---
title: Agentic Loop Practices
weight: 40
---

<!-- harnez:bundled -->
# Agentic Loop Practices — Multi-Agent Sprint Workflow

The owner's rules for multi-agent development loops, with the reason or evidence behind each.
Normal engineering practice is assumed and not restated. Command details live in the tools:
`harnez agent start --help`, `harnez issues --help`, `harnez read --help`.

## 1. Core Invariants

1. **Parallel Read, Sequential Write**
   - Many agents may read, grep and analyse at once; only one may modify files or run build
     mutations in a workspace at a time.
   - Sequential dispatch is the default for every task type, not only for overlapping code edits.
     Two "read-only" helpers still race on a shared, sequentially allocated resource: both can
     pick the same next ticket number from stale snapshots. Dispatch one subagent at a time unless
     the user asks for parallel execution of a specific task, and then check afterwards for this
     class of race.
2. **Canary & Test-Driven Verification**
   - Verify every task with real test runs or canary probes before declaring it done.
   - **External-contract tests** (scp/ssh operands, systemd sandbox behaviour, sample ages against
     real time, real CLI output or hook JSON) take their expected value from the real tool, a
     recorded real sample or the real clock, never from the code under test. Where unit tests
     cannot run the real tool, add a canary or container-tier check. Evidence: 934 M1 asserted a
     shell-quoted scp target and the live upgrade failed; 932 M2 pinned `observed_at` in a
     one-shot test; 919 used an injected runner that hid a sandbox ssh failure.
3. **Zero Zombie Guarantee**
   - Track and stop every background process, timer and subagent before the session ends.
   - Background tasks and jobs get `HTO=0` (`HTO=0 make test-q1`, `HTO=0 harnez ...`); otherwise
     `harnez exec`'s default 60-second timeout kills them. Then stop calling tools and wait for
     the completion notification: no polling, no `sleep` loops.
   - Hung or killed quota-1 test processes: `harnez clean procs q1 --kill`. Quota state is
     released only after the process group is verified gone.
4. **Responsive Host Orchestrator**
   - The host stays the user's coordination surface while workers run. "Hand this to a subagent"
     means delegate and stay responsive, not block on a wait call.
   - Start workers in the host background runner (see Background workers below). Wait for a
     child only when the user asks or the next user-visible step cannot proceed without it.
5. **In-Repository Single Source of Truth**
   - Tickets, decisions, retros and specifications live in git, not only in chat.
   - Use `issues/`, `docs/studies/` and `docs/feedback/` when present, otherwise an existing doc or ticket.
   - Start session notes at kickoff and update them after major work and after friction.
   - Tickets carry the work; messages only nudge. Put decisions and requests in a ticket, in
     whichever repository owns the work, and send only a pointer:
     `harnez agent send --name <session> "see ticket N in repo X"`.
6. **Context Discipline**
   - Never whole-file-read `AGENTS.md` or other files already in the active prompt. In-file
     warning banners do not help once the file is in the history.
   - For files of 100 lines or more use `harnez read -L <range>` or `harnez read -n`
     (`harnez read -I` is paused until issue 543 is fixed). Hooks may enforce this when
     `reading_discipline.enforce` is set in `~/.harnez/config.yaml` or via `HARNEZ_READ_ENFORCE`.
   - Global instruction files (`~/.claude/CLAUDE.md`, `~/.prime/agent/AGENTS.md`) cite docs as
     plain text ("see docs/Bash.md, Directory scoping"), never as `@docs/...`. Claude Code
     inlines an `@path` into every session of every project. Keep `@docs/...` for a project's
     own `AGENTS.md` doc list.
   - Apply the rule summaries in `AGENTS.md` directly; open a language doc by range only when
     you need a template or an unusual construct.
7. **Media & Demo Verification Gate**
   - **Always ask the user for explicit confirmation** that recorded output (reels, WebM demos,
     terminal recordings, screenshots) matches their expectation before publishing or embedding
     it. Recordings fail silently: invisible typing, missing frames, rendering artifacts.
8. **Deployment Transparency (remote-deploying projects only)**
   - **Local State** (checkout, configs, unit tests), **Deployed Artifact State** (remote
     binaries, permissions, config overlays) and **Active Daemon State** (remote process table,
     systemd units, crontab) are independent. A clean local build, a passing local test or a
     clean transfer exit code is evidence about Local State only. Probe the live host.
   - Such projects install the `deployment-transparency` doc for probes and anti-patterns.
9. **One-Level Delegation**
   - Every `harnez agent` session has a role, `--role orchestrator|developer|reviewer|advisor`
     (default `developer`); harnez adds the role's rules to each turn and exports
     `HARNEZ_AGENT_ROLE`.
   - The orchestrator coordinates and never codes. It starts developers, reviewers and advisors
     (one writer at a time) and deletes them when done. It never starts another orchestrator.
   - Developers, reviewers and advisors are leaf workers: no `harnez agent`, no native
     subagents, no delegating skills. harnez refuses `start`, `resume`, `stop`, `delete`,
     `compact` and `chat` for them; `list`, `status` and `models` stay available. Live checks
     that need an agent session are run by the orchestrator.
   - An explicitly named `provider:model:tier` is dispatched exactly through
     `harnez agent start`. If it is unavailable or fails, report the requested spec and ask.
     Never substitute the host model or a native subagent.
   - **Completion Evidence**: before accepting a "done" report the host checks the claimed
     changes with `git show --stat <commit>` and inspects the built output. A worker that closes
     tickets cites the commit that changed non-issue files.
   - A Codex worker quarantined after a usage-limit interruption is replaced by one named
     worker that gets the ticket state and last handoff. Committed work is complete and never
     replayed; uncertain in-flight work goes to the operator.
10. **Disk Headroom**
    - Before a download or build of a few GB (model weights, images, datasets), run
      `df -h <target dir>` and keep at least 10 GiB free. If space is short, use a scratch disk
      the user or project names, or ask. After `ENOSPC`, delete only the partial files that
      attempt created.
11. **Study & Retro Ticket Extraction**
    - A study or retro is complete only when every actionable follow-up is a ticket listed in
      it by number, or is marked as a non-goal with a reason. File findings owned by another
      repository there and record repository and number.
12. **Repository Scope & Product Primacy**
    - The applications and code in the current repository are the sole product of the session.
      Never offer or start implementation in another repository; only the user may order and
      scope cross-repository work. Elsewhere, read for context or file a requested or blocking
      issue.
13. **Steered Model Escalation**
    - Start with `--role dev --model lean`; escalate to `std` or `smart` on struggle, failed
      verification or ambiguity, then return to `lean` for the next step. Never trade
      verification for speed.
    - Steer the plan before granting write authority: the first prompt asks for a read-only
      plan, `resume` grants the write. No prompt template, so stored first prompts show each
      agent's own practice.
14. **Immediate Product-Issue Capture**
    - File larger obvious product issues at once (high priority when sprint-blocking); fix
      small local ones directly.

Session conduct:

- Decide low-risk, reversible choices yourself, record choice and reason under `## Decision`
  in the ticket, and report it in one line naming that location. Ask only about destructive or
  irreversible actions, unauthorized outward-facing actions, breaking API changes and security
  choices with real exposure.
- Limit plans and promises to this session or its next steps. Turn a lasting lesson into a
  check, a rule or prompt line, or a ticket, and name the safeguard and where it lives.
- TUI layout work: a checked-in `.ansi` mockup under `docs/data/` is the visual spec; the host
  drafts it and dispatches implementation against it (`TUIDesign.md`).
- Session skills: `/focus` re-grounds on this repository's priorities; `/decide` gives
  session-scoped autonomy (routine answers, advisors for second opinions, push ready work; no
  release); `/trial` reports a capability checkpoint; `/peer-assistant` handles explicit
  requests from another session through native messaging and is no always-on watcher.

### Background workers

`harnez agent start` and `harnez agent resume` are synchronous and print the agent's reply.
Run them directly for sequential work. To keep the chat free, run each as one job in the
**session background** (owned by your chat session, visible in its task list, wakes you on
exit) with `HTO=0`. Never use the **machine background** (shell `&`, `nohup`, `setsid`): the
session cannot see it and would have to poll. The tool and parameter differ per host (Claude
Code: Bash `run_in_background: true`, jobs stop after 30 minutes); `harnez agent start --help`
lists them and `harnez agent selftest --step hello` walks a host through the flow. Tell the
user the job id and session name. Use `harnez agent wait <session>` only to recover a lost job.

Delegation surfaces differ by host (for example `collaboration.spawn_agent` creates an entry
in the Agents view). They are status surfaces: the host still owns reporting, review,
integration and termination.

## 2. The 5-Phase Sprint Loop (`/sprint`)

### Phase 1: Parallel Advisory Discovery (read-only)

- Read-only advisors (one per ticket or area) audit requirements, find existing
  implementations and return findings with `file:line` and a plan. They never write, run
  mutating commands or spawn side effects.
- **Code over docs**: verify claims about behaviour in the code; docs may be stale. The host
  puts known code facts that contradict a doc into the handoff.
- **Kickoff & Commit Policy**: settle commit authority up front. Under an ask-first harness,
  ask for permission to commit verified checkpoints so the user can walk away. Agree whether
  subagents commit or return diffs.

### Phase 2: Sequential Development

- One ticket at a time, tests alongside or before the change, workspace green at every step.
- **Milestone-Boundary Atomic Commits**: commit each milestone once it passes verification and
  review, before starting the next.
- **Nuance Buffer**: collect non-blocking nuances in one refinement ticket instead of
  micro-review round trips; sweep it early only when it reaches an architectural tipping point.
- **Repro-before-fix for defect-shaped tickets**: for bug, timing, deadlock and race tickets,
  build a reproduction that asserts a concrete numeric baseline before writing the fix, and
  verify against that number. A fix can pass every existing gate and still miss the defect.
- **Commit stale/failed work before discarding it**: when an attempt is abandoned, do not open
  with `git checkout --`, `git reset --hard` or `git stash drop`. Commit it (current branch or
  a throwaway such as `git branch attempt-2-endpoint-cone`), then revert. The failed attempt
  stays diffable instead of surviving only as prose. Skip this only for trivial experiments
  where the description is the diff ("tried threshold=50 and 25, both failed").

### Phase 3: Pre-Commit Review Gate

The reviewer (a fresh subagent, or a dedicated pass) reads the diff and the assertions.

- **Test Assertion Rigor**: tests assert specific outcomes instead of merely executing code.
- After a developer's green full run, rerun only the touched packages (`go test <pkgs>`); run
  a full suite only if the developer ran none, at release, or with the user's authorization.
- **Docs & Ticket Sync**: status tags, indexes and docs match the code.
- **Backward Compatibility & Invariants**: project invariants and CLI boundaries hold.
- **What would I have done differently?** Look for edge-case omissions, runtime overhead and
  future debt; separate blocking issues from buffered nuances.
- **Live/Real-Environment Verification** for anything that installs a live hook, writes global
  config or resolves state from the environment (branch name, session variables, cwd). Green
  unit tests are **Unit-Test-Only Confidence**: fixtures supply explicit arguments and temp
  dirs that hide exactly the failures real use hits. Require one real end-to-end check after a
  genuine restart or re-apply. Evidence: eight tickets shipped on 2026-08-31 with good unit
  tests while automatic capture did not work at all in real use.
- **Root Cause vs. Symptom**: for a defensive fix (tolerant parsing, retries, swallowed
  errors), ask whether the source of the bad input can be fixed with a flag, a setting or a
  different invocation, and verify that against the real tool. Several review rounds finding
  new edge cases in the same defensive code mean: go back and fix the cause.
- **Exit condition**: the phase ends when the work is committed or the user has been asked to
  authorize the commit. With standing authority, commit now. Moving on with a clean, reviewed
  diff is **Reviewed-But-Uncommitted Carryover** (twice in one `weg` session, issues 044 and 046).

### Phase 4: Process & Subagent Hygiene

- Inspect background tasks; stop finished, idle and lingering jobs, timers and watchers; stop
  and delete child agents.
- Record a heartbeat: `harnez rate --ok "<note>" [<ticket_id>]`.

### Phase 5: Retrospective

- Record friction, harness observations and recommendations in the project's durable
  feedback or study location, or in a ticket.
- **Ticket Extraction gate** (Invariant 11): file each follow-up with `harnez issues new` and
  list the numbers in the study.
- Sync the tracker with `harnez index` and `harnez status`; never hand-edit index rows.
- **Single Status field per ticket**: check the whole file for more than one status-bearing
  field and update all of them, or standardize on one.
- **Closing gate**: flip every ticket whose work is shipped and verified to `Closed` before
  the session ends. A green build and a commit do not close a ticket. See `IssueTracking.md`,
  "Closing Is Part Of Done".
- Run `git status` before writing the retro: uncommitted reviewed work is a finding.
- **Evergreen trigger**: after a long goal, or once 5+ tickets were closed in the session, run
  `/evergreen` or propose it, so learnings reach the evergreen docs.

## 3. Lean and Reverse Sprints

The skills carry the steps; this section fixes the roles.

| Workflow | Use for | Host | Workers |
|---|---|---|---|
| `/sprint` | multi-ticket sprints, major features, broad refactors | plans and coordinates | advisors, one developer, independent reviewer |
| `/lean-sprint` | one focused ticket | zero coding, no exploration, diff-first review | Cheap Searcher, Focused Developer |
| `/lean-sprint-v1` | legacy single-developer handoff | zero coding, inline review | one developer |
| `/reverse-sprint` | routine, cost-sensitive coding | is the cheap developer and codes | reviewer per milestone, advisor only when stuck |

**`/lean-sprint`**

- The host never edits source or tests, does not browse or grep broadly, and reviews the
  developer's commit diff, test output and ticket. Decisions and refinements stay in the one
  target ticket.
- Cheap Searcher (`--role advisor --model cheap`): bounded read-only audits; returns paths and
  line ranges; no edits, commits or state-changing tests. Pass only relevant findings on.
- Focused Developer (`--role dev --model lean`; `std` for moderate, `smart` for difficult,
  `fast` for easy, `cheap` for low-cost work): test-first, verification, one commit per
  milestone. Use prompt files for long handoffs instead of shell-interpolated prompts.
- Escalate to an independent reviewer only for cross-subsystem risk, missing test coverage or
  unexpected complexity.
- Keep a running report in `docs/studies/` when present, otherwise in the ticket. Name it
  with a date prefix and `two-agent-lean-sprint-<slug>`; record searcher findings, milestones
  and commits, verification, and token and time observations. Finish with helper teardown,
  ticket status, `harnez index` and `harnez rate --ok`.

**`/reverse-sprint`**

- A cheap developer (`--role dev --model cheap`) leads and writes code and tests.
- It compacts every 100–150k tokens or at milestone boundaries, after saving notes to the
  ticket; low-tier models degrade with long context.
- A reviewer (`--role reviewer --model smart`) inspects `git diff HEAD~1` per milestone.
- An advisor (`--role advisor --model smart`) is called only when stuck and gets bounded
  context (file paths, line ranges, a precise question), so it does not scan the repository.

## 4. Calibrated Friction Reporting

- **Substantive sessions only**: report tool, environment or sandbox friction after sessions
  with real hurdles.
- **No repeated noise** about known, trivial quirks.
- **Actionable root causes**: blocker, failure mode, workaround tried, recommended fix.

**Name items in reports**: in every report (host to user, worker to host, ticket delivery
notes) give each milestone, ticket or step a short name at least once: "M3 (single-write
settings removal)", "498 (Claude resume inside Claude Code)", not a bare "M3" or "498". The
reader does not hold the milestone list in their head.

## 5. Anti-Patterns to Avoid

Rules above are not repeated here; these are the failure modes they do not name.

- **Goals Without Exit Clauses**: a `/goal` without an exit clause deadlocks the Stop hook
  when progress needs user input. Every goal carries a circuit breaker: on a looping worker, a
  hard architectural or OS blocker, or no progress, clear the goal and write a failure report
  (what was tried, why it failed, the resume point) in the ticket.
- **Fixing Without Retrying the Stuck Case**: declaring a bug fixed because new unit tests
  pass. In harnez 673 three stacked bugs hid behind one error message, and only a live resume
  of the stuck worker showed the next one. Re-run the real failing session, file or request
  after every fix; if it still fails, feed the exact production input to the code under test.
- **Shell `&` Detaching / Machine-Background Workers**: `&`, `nohup`, `setsid` and `disown`
  put a job outside the harness: no tracking, no stop button, no completion signal. Use the
  host's own background facility (see Background workers).
- **Blocking `sleep` Waits**: a long `sleep`, or a loop of short ones, to wait out CI, a
  deploy, a queue or a background process burns the turn and leaves no record of what was
  awaited. Let tracked background work notify you. For genuinely external state use the
  host's scheduled wakeup or loop mechanism, with an interval matched to how fast the state
  changes. Never chain sleeps to get around a restriction on blocking sleep.
- **Chat-Visible Empty Polling**: re-checking a long job on a short fixed interval and
  reporting "still running". Prefer a completion signal; without one, launch the work so it
  writes a log or result file, give the user the job id, log path and expected duration, and
  return control. Surface events (started, first output, exited, artifact written), not
  heartbeats.
- **Buffered Long-Running Output**: a long build or test piped through `tail`, `grep`, `sort`,
  `wc` or `head` prints nothing until the pipeline exits and looks stuck. Run it plain, or use
  `cmd 2>&1 | tee <log>` when a trimmed summary is wanted.
- **Chatty Watch Wrappers**: poll-and-redraw tools (`gh run watch`, `docker logs -f`) wrapped
  in a routine `make` target flood the context. Query status (`gh run view --json status`) on
  a matched interval and print one summary line at the end.
- **`cd`-scoped commands**: `cd <dir> && git status` leaks the cwd into later calls. Use
  `git -C`, `make -C` or a subshell; see `Bash.md`, Directory scoping.
- **Stopping an Unverified Process**: require a known start time for every process root before
  sending stop messages or signals; sessions without identities and hosts that cannot read
  start times fail closed. Scanning and signalling are not atomic, so PID reuse can still race.
- **Baking Real Credentials In For A Fast Dev Loop**: real hostnames, MACs, subnets or
  credentials hardcoded "temporarily". `smarthome` needed a full history-sanitization pass
  before its public release. Start with RFC-1918 or example values and a credential-source
  seam, and wire a secret scanner into `check`/`test` from the first commit.
- **Narrow String-Substitution Edits**: prefer `apply_patch` or whole-block replacement. In
  `smarthome`, `Edit` failed in 11.1% of 108 calls and `apply_patch` in 4.2% of 24.
- **Prompt Micromanagement**: restating base rules, tool definitions or style guides that the
  harness prompt already carries in a subagent dispatch.
- **Rubber-Stamp Reviews**: a review pass that inspects neither assertions nor diffs.

<!-- harnez:model-picks:begin -->
These are the shipped picks. User models.yaml overrides are listed by `harnez agent models --picks`.

| Role | Trait | Meaning | Chain | Default |
|---|---|---|---|---|
| advisor | cheap | Low-cost work. | luna:low, haiku:low | yes |
| advisor | smart | Difficult judgment. | opus:low, opus:med, sol:low |  |
| developer | ai | AI-oriented work. | unset |  |
| developer | cheap | Low-cost work. | luna:med |  |
| developer | fast | Fast work. | haiku:med |  |
| developer | gui | Graphical interfaces. | lean, std |  |
| developer | lean | Efficient standard work. | terra:low | yes |
| developer | smart | Difficult judgment. | sol:med, opus:med |  |
| developer | std | Standard work. | terra:med, opus:low |  |
| developer | tui | Terminal interfaces. | smart |  |
| reviewer | smart | Difficult judgment. | sol:low | yes |
<!-- harnez:model-picks:end -->
