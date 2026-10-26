# Two-Agent Lean Fresh-Handoff Sprint

Orchestrate a lean, milestone-driven sprint with a cheap read-only searcher, optional read-only architect refinement, focused developer, strict zero-coding review, and single-ticket pre-work batching.

Reference Practice: `@docs/AgenticLoop.md`

## Role Contract — Direct Execution

You, the agent that received this invocation, are the Host Orchestrator for this task.
Run the complete lean workflow below yourself in this session: goal handoff, sequential
milestone dispatch, concise pre-commit/milestone review, and teardown with status sync.

### Strict Invariants for the Host Orchestrator

0. **One-Level Delegation**: start helpers only as `--role advisor|developer|reviewer`; they are leaf workers and never call `harnez agent`. Never start another orchestrator or hand the sprint to another agent.

1. **Zero Coding**:
   - **The Host Orchestrator NEVER writes code, edits source files, or applies "quick fixes" directly.**
   - All code implementation, file editing, test creation, and bug fixing are strictly executed by the dispatched developer agent.
2. **Diff-First Inspection (No Exploratory Code Digging)**:
   - **The Host Orchestrator NEVER performs exploratory repository digging**: no broad grep, whole-file reading, API/spec research, or call-graph tracing.
   - The host reads the task and bounded reports, then reviews the developer's **commit diff** (`git log -n 1 --stat`, `git diff HEAD~1`) and test output. Discovery belongs to the Searcher, except for the bounded Developer plan permitted by §1; implementation discovery belongs to the Developer.
3. **Single-Ticket Communication (No Follow-Up Tickets)**:
   - **The single target issue ticket (`issues/XXX-....md`) is the sole medium of communication between the host and developer.**
   - Do **NOT** create secondary nuance refinement tickets (`#XXX-refinements`) or splinter follow-up tickets for lean sprints.
   - Discovered nuances and fixes from milestone $N$ are written directly into the ticket under milestone $N+1$ as **Pre-Work**. Never dispatch isolated micro-tasks to the developer.
4. **Low-Risk Decisions**: Decide simple, low-risk or reversible choices yourself; record each choice and reason under `## Decision` in the ticket and report it in one line naming that location. Ask only about destructive or irreversible actions, unauthorized outward-facing actions, breaking API changes, or security choices with real exposure; model choices remain the user's.

---

## Invocation Syntax

- `/lean-sprint <ticket-numbers>` (e.g. `/lean-sprint 064` or `/lean-sprint 042, 043`)
- `/lean-sprint "<scoped-task-or-bug-description>"`
- `/lean-sprint <ticket> refined <tier>` (or `with architect`, `with advisor`, `with sol:low`, `using opus:low`)
- The legacy single-developer workflow is available as `/lean-sprint-v1 <ticket-or-task>`.

---

## Lean Milestone-Review Workflow

For focused, milestone-based tasks, execute this fast-path, token-efficient loop:

### 1. Bounded Discovery with the Cheap Searcher
- Start with the ticket and dispatch a read-only Searcher (`luna:low` / `luna-search` / `haiku:low`, `--role advisor`) for a bounded audit: verify the ticket premise, locate relevant files and exact line ranges, answer specific questions, explore APIs/specs, or search the web. Give a small question list and return format; do not ask it to implement, edit files, run mutating commands, or commit. Use a prompt file for longer questions.
- Skip the Searcher when the ticket was just refined against live code in this sprint and already cites exact file:line causes; say so in the sprint report.
- Also skip the Searcher for a narrowly scoped ticket that identifies the exact file and function, documents the complete expected write set (including tests/docs), and leaves no unresolved premise, scope, or architecture questions. Require a plan-only first Developer turn that inspects live code, verifies the premise and current locations, and confirms the write set; file:line citations alone do not qualify. The host must review the concrete findings before granting writes and reconsider developer/model choice. If inspection reveals stale assumptions, missing files, or scope uncertainty, run the Searcher before implementation. Record the skip and findings in the sprint report.
- When used, run the Searcher sequentially before the Developer. The host reads its concise findings and decides what the Developer needs; the host does not repeat discovery by browsing the repository.

### 1.5. Optional Architect Refinement
- When the user asks for a lean sprint "with an advisor", they mean this architect step, not the Searcher (both run as `--role advisor`).
- Run this step when the user explicitly requests refinement or confirms the host's proactive suggestion. If refinement was not requested, suggest it when Searcher findings show complex architecture such as multi-system boundaries, API/schema redesign, or non-trivial component trade-offs.
- The architect tier must be one of `opus:low`, `opus:med`, `sol:low`, `sol:med`, or `argon:low`. Normalize bare `opus`, `sol`, or `argon` to the matching `:low` tier. If refinement is requested without a valid tier, stop immediately, list the supported tiers, and ask the user to choose before starting any agents.
- Dispatch the architect as `--role advisor` with read-only instructions. It returns a structured architecture, milestones (`M1`, `M2`, ...), boundary constraints, edge cases, and pre-work requirements; it does not implement, edit files, or run mutating commands.
- The host records the architect's milestones, boundaries, edge cases, and pre-work in the target ticket before handing it to the Developer. The host remains subject to the Zero Coding invariant above.

### 2. Goal Handoff to Focused Developer
- The Host Orchestrator dispatches one developer worker (`haiku:high` for well-specified implementation and tests; `sol:low` when judgment, orchestration, or stronger reasoning is needed) using `harnez agent start --role developer --name <worker> --model <model> -d <dir> -f <prompt-file>` (or the active dispatch method), run with `HTO=0` as one session-background job per the host table in `harnez agent start --help` (Claude Code: Bash with `run_in_background: true`). Use a prompt file for long or complex handoffs instead of a long inline shell argument. Report the job id and session name to the user; the job's exit is the only completion signal, so never promise a review without it. Luna is for background or mechanical work, not Go development; avoid agy models for developer work, except `argon:low`. Keep Haiku prompts below 100k tokens where possible: Haiku 5.5 requests above 100k prompt tokens cost more per token ([Claude model configuration](https://code.claude.com/docs/en/model-config#haiku-55-context-window)).
- An explicitly named `provider:model:tier` must be dispatched exactly through `harnez agent start`; on failure, report it and ask for guidance rather than substituting the host model or a native subagent.
- Provide these facts in every handoff (a required checklist, not a wording template; missing items caused most follow-up turns):
  - Ticket number and goal, with its acceptance criteria.
  - Files to change, all of them (including doc copies and sibling docs).
  - Files not to touch (other sessions' uncommitted work) — never stage them.
  - Tests already known to fail and why, so the worker neither fixes nor blames them.
  - Edge and negative cases to test (ambiguous input, near-miss values).
  - Test requirements (reproduction test first for bugs, unit tests for features).
  - External-contract tests: expected values from the real tool, see AgenticLoop.
  - If `.harnez/repo.yaml` sets `issues.tracker: none`, follow `.harnez/rules/Index.md` for the local-tracker/session fallback.
  - The exact close command with a written reason: `harnez issues close N "<real outcome>"`; no placeholders. The close refuses until a commit outside `issues/` names N; `--no-code` only for a ticket that needed no code.
    Close only after the ticket's acceptance check passes; the reason states the decisive evidence (the
    key numbers or check result), not a pasted log. When acceptance is not met, leave the ticket open and
    report. (Sprint 850: closed with a false match still above the cutoff.)
  - Never amend, autosquash or rebase commits; fix with new commits. Other sessions push to the same main.
    (Bug-hunt 2026-10-22: an autosquash rewrote pushed commits and blocked every ticket commit.)
- **Plan First (read-only)**: Recommended: start the worker's initial prompt with a read-only planning step ("read-only: plan ..."; no edits until the host has seen the plan), then `resume` to grant write authority. Give no wording template for the prompt beyond the checklist above: agents have their own best practices, and the stored first prompts (telemetry DB) are how we observe and compare them. In one-shot mode a plan request alone is not enforced, so a plan-only first prompt is the reliable form. Codex workers often end the plan turn right after `CONFIRM:` without inspecting (seen twice on 2026-09-30): ask for "inspect now and reply with the concrete plan (files, schema, checks, commits) in this turn", and if the reply is only a plan-to-plan, resume once with that request. This step is mandatory when using §1's Developer-plan discovery exception.
- **Trust the Base Framework**: Do not duplicate system prompts or micromanage formatting conventions.
- **Reading Discipline**: Instruct the developer to use line-bounded reads (`harnez read -L <range>` / `-n`) for medium/large files (`harnez read -I` is paused until issue 543, a memory blow-up, is fixed).
- **Stay Responsive**: Dispatching the worker must not block the chat; note the emitted Reconnect Banner and return control or proceed with review preparation.

### 3. Autonomous Milestone Execution & Commit
- The developer agent implements the milestone autonomously.
- Follows Test-Driven Development (TDD) and executes repo-native verification (`go test ./...`, `make test`).
- Under a one-run test budget, use `make test-q1` when available; otherwise run `harnez exec --quota-1 -- <repo-test-command>`. Write full output to a file and grep for `--- FAIL`; never pipe it into `tail`. Before committing, quote the run's final test summary line in the report: no `--- FAIL` in a run that stopped early (e.g. at gofmt or vet) is not a pass (941 M4 committed after a run that never reached the tests). A failure in an untouched test gets its own ticket instead of a loosened assertion. After fixing a failure from that run, rerun only the touched packages and report that the full run is owed; the host runs it before review (issue 896). If that host run fails, the host fixes nothing: it records the failure as pre-work in the ticket and resumes the developer.
- Immediately commits the verified milestone at the boundary (`git commit -m "... (issue XXX MX)"`).
- Reports completion back to the Host Orchestrator. Name each milestone or ticket at least once with a short label, e.g. "M3 (single-write removal)", not bare "M3" (`@docs/AgenticLoop.md` §4, Status reports).

### 4. Concise Milestone Review & In-Ticket Pre-Work Embedding
- Upon developer milestone completion, the Host Orchestrator performs a rapid, diff-only inspection:
  - Run `git fetch` and `git status -sb` first: a local main that diverged from origin means history was
    rewritten or another session pushed; resolve that before reviewing.
  - Check `git log -n 1 --stat`, `git diff HEAD~1`, and run verification tests.
  - After the developer's green full run, rerun only touched packages (`go test <pkgs>`); run a full suite only if the developer did not run one, at release, or with the user's authorization.
  - Evaluate test assertion rigor, ambient environment leaks, and edge-case omissions directly against the diff.
  - The host/reviewer reads every changed assertion line in test diffs; a "fixed the test" report can hide a
    weaker check (884: `HasPrefix` → `Contains` dropped the ordering the test name promised).
  - **Plausibility check for measurements**: when the milestone produces numbers (reports, stats,
    benchmarks, token or quota figures), run the real command and check the values make sense, not
    only that tests pass: units and magnitudes against a known reference or hand calculation, totals
    that add up, parts that don't exceed the whole, no field holding another field's data, no silent
    zeros for missing data, and no rows that vanish. Record implausible values as pre-work for the
    next milestone. (Sprint 519: green tests hid "new input" holding total input and a 43% drain on
    one session.) Before a reviewer's number becomes an expected value in pre-work, check it against
    the raw source. (878 M3: pre-work said "expect 119" from a reviewer count; the raw JSONL gave 128.)
  - For user-visible output, the host runs the installed binary itself (several runs, a pty for
    interactive views) and compares every expected row. A developer's pasted output or "closed" is
    not evidence. (Sprint 657: a dev closed the ticket claiming all providers rendered; live compact
    showed none of Claude/Codex, and rows varied between runs.)
    The same holds for any output form: when the evidence is web screenshots, the host opens each
    variant itself (dark scheme, phone width), and variants come from the real user setting (the
    browser's color scheme), not one forced inside the page. (harnez.org 028 M2.4: a dev reported 32
    screenshots fine; every "dark" one was light because only a forced `data-theme` was ever tested.)
  - Test services the first minute after start to surface readiness, catch-up, and heartbeat bugs.
    (Sprint 714: `harnez usage` hung after every server restart; tests always started with data.)
  - When the change touches generated, rendered or git-integration files, the host runs
    `make install && harnez init -d .` and requires a clean `git status`; for git integration it
    also runs the real flow in a scratch repo and checks `git show HEAD:<file>`. (Sprints 843/847:
    init reverted M3's rules edit; green tests hid a driver that init never installed and merge
    commits missing rows.)
- **Strictly No Direct Fixes & No Micro-Task Rounds**:
  - The host does **NOT** modify code files.
  - The host updates the ticket: records milestone $N$ delivery summary, naming the milestone ("M2 delivered: flag and schema gating").
  - If adjustments, test hardenings, or nuances are required, the host documents them directly in the ticket under Milestone $N+1$ as **"Pre-Work / Required Refinements"**.
  - Commit the ticket update immediately. Resume the developer only after that edit and commit
    succeeded, in a later call, never chained in the same batch. (878: a pre-work edit on a wrong path
    nearly sent the developer to a section that did not exist.)

### 5. Milestone Advance or Completion
- The developer agent picks up the updated ticket for Milestone $N+1$ (resumed via `harnez agent resume --name <session_id> "<pre_work_and_milestone_prompt>"` or native subagent message), executes the embedded pre-work first, and then proceeds with Milestone $N+1$ implementation.
- Repeat Steps 1–3 for each milestone.
- Once the final milestone passes concise review and tests are 100% green, confirm completion.
- Before a ticket closes, the host/reviewer compares what was delivered with the ticket's `/goal` and the
  milestone plan; a shortfall is parked in its own ticket and named in the close reason (884 M9 moved 157 of
  ~5k planned lines and was first closed as delivered).

### 6. Teardown, Study Report & Status Sync
- Terminate the developer subagent and every research or probe helper (`harnez agent delete --name <session_id>` or `manage_subagents kill`) and drain background jobs; `harnez agent list` should show none left.
- If `.harnez/repo.yaml` sets `issues.tracker: none`, follow `.harnez/rules/Index.md`; otherwise update ticket status and refresh the Harnez index.
- Maintain a running report in `docs/studies/` when enabled by `.harnez/repo.yaml`; otherwise follow `.harnez/rules/Index.md` for the local-docs/session fallback. Use a date-prefixed `two-agent-lean-sprint` filename and short task slug, with front matter `kind: sprint` (see `/study`). The Host Orchestrator records the Searcher questions/findings, Developer milestones and commits, verification outcomes, and observed token/time efficiency as the sprint progresses; do not reconstruct it only at the end.
- Output a brief, high-level summary of delivered milestones and verification status. Name each milestone or ticket at least once with a short label, e.g. "M3 (single-write removal)", not bare "M3" (`@docs/AgenticLoop.md` §4, Status reports).
