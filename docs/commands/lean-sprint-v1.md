# Lean Fresh-Handoff Sprint (v1)

- Preflight: grep the files/strings named by the ticket, use `harnez read` and `git log -S` on what it says exists, and
  verify its premise still holds on HEAD. If already done or obsolete, close the ticket with the finding and dispatch no developer.

Orchestrate a lean, milestone-driven developer handoff with strict zero-coding review loops and single-ticket pre-work batching.

Reference Practice: `@docs/AgenticLoop.md`

## Role Contract — Direct Execution

You, the agent that received this invocation, are the Host Orchestrator for this task.
Run the complete lean workflow below yourself in this session: goal handoff, sequential
milestone dispatch, concise pre-commit/milestone review, and teardown with status sync.

### Strict Invariants for the Host Orchestrator

0. **One-Level Delegation**: start helpers only as `--role developer|reviewer|advisor`; they are leaf workers and never call `harnez agent`. Never start another orchestrator or hand the sprint to another agent.

1. **Zero Coding**:
   - **The Host Orchestrator NEVER writes code, edits source files, or applies "quick fixes" directly.**
   - All code implementation, file editing, test creation, and bug fixing are strictly executed by the dispatched developer agent.
2. **Diff-First Inspection (No Exploratory Code Digging)**:
   - **The Host Orchestrator NEVER performs whole-file exploratory reading, multi-file browsing, or deep call-graph tracing.**
   - Orchestrator inspection is strictly bounded to:
     - The target ticket / issue description.
     - The developer agent's **commit diff** (`git log -n 1 --stat`, `git diff HEAD~1`).
     - Test execution outputs (`go test ./...`, `make test`).
   - Deep codebase exploration is the worker's job; the host conserves context and token budget for high-signal evaluation.
3. **Single-Ticket Communication (No Follow-Up Tickets)**:
   - **The single target issue ticket (`issues/XXX-....md`) is the sole medium of communication between the host and developer.**
   - Do **NOT** create secondary nuance refinement tickets (`#XXX-refinements`) or splinter follow-up tickets for lean sprints.
   - Discovered nuances and fixes from milestone $N$ are written directly into the ticket under milestone $N+1$ as **Pre-Work**. Never dispatch isolated micro-tasks to the developer.
4. **Low-Risk Decisions**: Decide simple, low-risk or reversible choices yourself; record each choice and reason under `## Decision` in the ticket and report it in one line naming that location. Ask only about destructive or irreversible actions, unauthorized outward-facing actions, breaking API changes, or security choices with real exposure; model choices remain the user's.

---

## Invocation Syntax

- `/lean-sprint-v1 <ticket-numbers>` (e.g. `/lean-sprint-v1 064` or `/lean-sprint-v1 042, 043`)
- `/lean-sprint-v1 "<scoped-task-or-bug-description>"`

---

## Lean Milestone-Review Workflow

For focused, milestone-based tasks, execute this fast-path, token-efficient loop:

### 1. Goal Handoff to Low-Cost Developer
- The Host Orchestrator dispatches a developer worker using `harnez agent start --role developer --name <worker> --model <model> -d <dir> "<milestone_prompt>"` (or the active subagent dispatch method), run with `HTO=0` as one session-background job per the host table in `harnez agent start --help` (Claude Code: Bash with `run_in_background: true`). Report the job id and session name to the user; the job's exit is the only completion signal, so never promise a review without it. Prefer `luna` first; use `haiku:high` for a simple, well-specified ticket with a machine-checkable acceptance test (the host reviews its diff closely); use `sol` when stronger judgment is needed; avoid `agy` models for developer work, except `argon:low`.
- An explicitly named `provider:model:tier` must be dispatched exactly through `harnez agent start`; on failure, report it and ask for guidance rather than substituting the host model or a native subagent.
- Provide these facts in every handoff (a required checklist, not a wording template; missing items caused most follow-up turns):
  - Ticket number and goal, with its acceptance criteria.
  - Files to change, all of them (including doc copies and sibling docs).
  - Files not to touch (other sessions' uncommitted work) — never stage them.
  - Tests already known to fail and why, so the worker neither fixes nor blames them.
  - Edge and negative cases to test (ambiguous input, near-miss values).
  - Test requirements (reproduction test first for bugs, unit tests for features).
  - If `.harnez/repo.yaml` sets `issues.tracker: none`, follow `.harnez/rules/Index.md` for the local-tracker/session fallback.
  - The exact close command with a written reason: `harnez issues close N "<real outcome>"`; no placeholders.
- **Plan First (read-only)**: Recommended: start the worker's initial prompt with a read-only planning step ("read-only: plan ..."; no edits until the host has seen the plan), then `resume` to grant write authority. Give no wording template for the prompt beyond the checklist above: agents have their own best practices, and the stored first prompts (telemetry DB) are how we observe and compare them. In one-shot mode a plan request alone is not enforced, so a plan-only first prompt is the reliable form. Codex workers often end the plan turn right after `CONFIRM:` without inspecting (seen twice on 2026-09-30): ask for "inspect now and reply with the concrete plan (files, schema, checks, commits) in this turn", and if the reply is only a plan-to-plan, resume once with that request.
- **Trust the Base Framework**: Do not duplicate system prompts or micromanage formatting conventions.
- **Reading Discipline**: Instruct the developer to use line-bounded reads (`harnez read -L <range>` / `-n`) for medium/large files (`harnez read -I` is paused until issue 543, a memory blow-up, is fixed).
- **Stay Responsive**: Dispatching the worker must not block the chat; note the emitted Reconnect Banner and return control or proceed with review preparation.

### 2. Autonomous Milestone Execution & Commit
- The developer agent implements the milestone autonomously.
- Follows Test-Driven Development (TDD) and executes repo-native verification (`go test ./...`, `make test`).
- Under a one-run test budget, use `make test-q1` when available; otherwise run `harnez exec --quota-1 -- <repo-test-command>`. Write full output to a file and grep for `--- FAIL`; never pipe it into `tail`. A failure in an untouched test gets its own ticket instead of a loosened assertion. After fixing a failure from that run, rerun only the touched packages and report that the full run is owed; the host runs it before review (issue 896). If that host run fails, the host fixes nothing: it records the failure as pre-work in the ticket and resumes the developer.
- Immediately commits the verified milestone at the boundary (`git commit -m "... (issue XXX MX)"`).
- Reports completion back to the Host Orchestrator. Name each milestone or ticket at least once with a short label, e.g. "M3 (single-write removal)", not bare "M3" (`@docs/AgenticLoop.md` §4, Status reports).

### 3. Concise Milestone Review & In-Ticket Pre-Work Embedding
- Upon developer milestone completion, the Host Orchestrator performs a rapid, diff-only inspection:
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
    one session.)
  - For user-visible output, the host runs the installed binary itself (several runs, a pty for
    interactive views) and compares every expected row. A developer's pasted output or "closed" is
    not evidence. (Sprint 657: a dev closed the ticket claiming all providers rendered; live compact
    showed none of Claude/Codex, and rows varied between runs.)
    The same holds for any output form: when the evidence is web screenshots, the host opens each
    variant itself (dark scheme, phone width), and variants come from the real user setting (the
    browser's color scheme), not one forced inside the page. (harnez.org 028 M2.4: a dev reported 32
    screenshots fine; every "dark" one was light because only a forced `data-theme` was ever tested.)
- **Strictly No Direct Fixes & No Micro-Task Rounds**:
  - The host does **NOT** modify code files.
  - The host updates the ticket: records milestone $N$ delivery summary, naming the milestone ("M2 delivered: flag and schema gating").
  - If adjustments, test hardenings, or nuances are required, the host documents them directly in the ticket under Milestone $N+1$ as **"Pre-Work / Required Refinements"**.
  - Commit the ticket update immediately.

### 4. Milestone Advance or Completion
- The developer agent picks up the updated ticket for Milestone $N+1$ (resumed via `harnez agent resume --name <session_id> "<pre_work_and_milestone_prompt>"` or native subagent message), executes the embedded pre-work first, and then proceeds with Milestone $N+1$ implementation.
- Repeat Steps 1–3 for each milestone.
- Once the final milestone passes concise review and tests are 100% green, confirm completion.
- Before a ticket closes, the host/reviewer compares what was delivered with the ticket's `/goal` and the
  milestone plan; a shortfall is parked in its own ticket and named in the close reason (884 M9 moved 157 of
  ~5k planned lines and was first closed as delivered).

### 5. Teardown & Status Sync
- Terminate the developer subagent and every research or probe helper (`harnez agent delete --name <session_id>` or `manage_subagents kill`) and drain background jobs; `harnez agent list` should show none left.
- If `.harnez/repo.yaml` sets `issues.tracker: none`, follow `.harnez/rules/Index.md`; otherwise update ticket status and refresh the Harnez index.
- Output a brief, high-level summary of delivered milestones and verification status. Name each milestone or ticket at least once with a short label, e.g. "M3 (single-write removal)", not bare "M3" (`@docs/AgenticLoop.md` §4, Status reports).
