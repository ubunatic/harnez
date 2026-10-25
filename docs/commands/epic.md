# Plan, Refine and Run an Epic

You, the agent that received this invocation, are the Host Orchestrator. An epic is one ticket
that owns a task table; each task becomes a subticket run as a `/lean-sprint`.

Invocation: `/epic "<idea>"` drafts a new epic; `/epic <n>` refines or runs epic ticket `<n>`;
`/epic <n> no-web` skips the web fanout.

## Dispatch

- Keep the explicit forms above unchanged. Accept modifiers before `/epic` (`file new /epic`,
  `run new /epic`, `next /epic`) or after it (`/epic file`, `/epic run`, `/epic next`).
- `file new` drafts and files an epic only. `run new` drafts, files, and runs it. `next` selects
  and runs the next logical epic or task in the current epic, using session context and its task table.
- Bare `/epic` checks open epics with `harnez find -d . issues -a status:active`, then keeps tickets
  whose metadata says `Category: Epic`. If any exist, ask whether to run one or group new work.
- With no open epic, group relevant session and active-backlog tickets by thematic affinity
  (shared subsystem, component, or context); use priority and tags to rank within a group, never
  to combine unrelated tickets. If affinity is ambiguous, propose the group and ask before filing.
- For a clear group, file the epic, adopt it as the current internal goal, then run it. Do not
  invoke a harness `/goal`; tell the user they can use one if they want a harness-level goal.
- Automode may make routine implementation choices; stop for owner confirmation before destructive
  deletion, breaking changes, or bypassing a canary.

Delegate, don't restate: `/issue` (filing), `/lean-sprint` (roles, dispatch, review),
`/story` (study shape), `/evergreen`, `/wrap`. If `.harnez/repo.yaml` sets
`issues.tracker: none` or `docs.studies: false`, follow `.harnez/rules/Index.md`.

## 1. Draft

File the epic with `/issue` (direct). It contains:

- `/goal` with an exit clause and the circuit breaker (step 6b).
- Out of scope, open decisions.
- Task table: ticket, what, sprint group, searcher → architect → developer, reviewer, status.
- Final tasks: `/evergreen` and `/wrap`.

Tasks stay placeholders until steps 2–4 have run.

## 2. Prior-Art Web Fanout (optional; default on for new features)

Goal: check the direction and avoid rebuilding established, stable, maintained, non-bloated tools.

- Start several cheap read-only searchers in parallel (`haiku:low`, `luna:low`, `--role advisor`),
  one angle each: existing tools, libraries to reuse, standards/specs, known pitfalls.
- Each returns candidates with source link, last release date, activity, dependency weight,
  license, and a fit verdict. No link, no finding.
- The host aggregates them into one study (studies dir when present, otherwise the epic ticket)
  and records a "build vs. reuse" decision per task in the epic.

Skip it for pure refactors or internal fixes; say so in the epic.

## 3. Code Fanout

Cheap searchers check the live repo for each task's premise, location and size. A task that is
not small, or hides an owner decision, moves into its own sprint group.

## 3a. Phases and Delivery

For multi-group epics, organize groups as prose phases with their dependencies, child tickets,
and exit gates; use a DAG when work can proceed independently. Start a phase only when its
dependencies' gates pass. Keep infrastructure and integration work after the standalone work
it depends on. An operational epic owns one repository: for sibling-repository work, file a
planning bridge here and an anchor ticket or follow-up epic in the owning repository.

Label each child `code` or `design/discovery`. Code work requires the ticket's tests, install,
and runtime canary before closure. Design/discovery work produces a study and a decomposed
backlog, and states that runtime delivery was not verified. A phase exits when its children
are closed, its declared canary or design review passes, and the working tree is clean.

## 4. Refine

Each task becomes a subticket (`/issue`) with acceptance, write set, and:

- Developer tier: haiku for well-specified, machine-checkable work; sol for judgment.
- Architect only when the design is open (`/lean-sprint` §1.5 tiers).
- Reviewer: opus/sol reviewer for risky work; orchestrator diff review for small work.
- Ask each allocated worker to report the allocated ticket number and confirm that every
  actionable study/retro follow-up was filed and linked; the study must list those numbers.

Mix vendors across a sprint. Update the epic's task table with the numbers.

## 5. Decisions

Batch owner questions once, each with a recommendation. When the owner delegates, decide
and record each choice in the subticket's `## Decision`.

## 6. Run

One `/lean-sprint` per sprint group; group very small tickets. Update the task table as each
lands (commit with the ticket update).

### 6b. Circuit Breaker

Every `/goal` this skill writes or runs carries one. When a worker loops, an architectural or
OS blocker is hard to solve, or the orchestrator sees no progress: clear the goal and write a
failure report in the epic (what was tried, why it failed, resume point).

## 7. Skip Rule

A subticket that needs new commands, unavailable resources or an owner decision nobody can
default is skipped and named in the epic, not forced.

## 8. Close

The epic closes when every task is closed, skipped with a reason, or parked in its own ticket.
The close reason lists each. Compare delivery with the epic's `/goal` first.

## 9. Evergreen and Wrap

Run the final tasks: sprint study, review-learning entries, follow-up tickets (`/evergreen`),
then `/wrap`. Report the epic number, closed/skipped/parked tasks, and the next command.
