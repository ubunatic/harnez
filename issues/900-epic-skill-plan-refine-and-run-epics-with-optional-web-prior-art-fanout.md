# 900 — epic skill: plan, refine and run epics with optional web prior-art fanout

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 899 (new-project skill uses it), 894 (epic run used as the worked example), docs/commands/lean-sprint.md,
docs/commands/issue.md, docs/commands/story.md, docs/commands/evergreen.md, docs/commands/Wrap.md, docs/IssueTracking.md

---

/goal A harnez-managed `epic` skill (docs/commands/epic.md + config.yaml `skills:` entry) drafts, refines and
runs epics: it files the epic ticket with a task table, optionally runs a web-search fanout to check the
direction and avoid rebuilding established tools, turns tasks into concrete subtickets with worker tiers, runs
them as lean sprints and ends with evergreen and wrap; verified by applying it to one real epic; stop and ask
when a subticket needs an owner decision the skill cannot default.

## Owner request (2026-10-24)

An epic skill for 899's flow and for epics in general. Epics get an optional web-search fanout to double-check
the direction and avoid reinventing well-established, stable, modern, non-bloated, maintained "wheels".

## Steps (owner's list, plus host-added details marked +)

1. **Draft**: epic ticket with `/goal` + exit clause, out of scope, open decisions, task table (ticket, what,
   sprint group, searcher → architect → developer, reviewer, status). + Final tasks: `/evergreen`, `/wrap`.
2. **Prior-art fanout (optional, default on for new features)**: parallel cheap web searchers (haiku:low,
   luna:low), each one angle; + each returns candidates with source link, last release date, activity,
   dependency weight, license, and a fit verdict; + the host aggregates into one study and a "build vs. reuse"
   decision per task, recorded in the epic.
3. + **Code fanout**: searchers check the live repo for each task's premise and size; tasks that are not small
   move into their own sprint (894: 866 and 871 hid owner decisions).
4. **Refine**: each task becomes a subticket with acceptance, write set, proposed developer tier (haiku for
   well-specified, machine-checkable work; sol for judgment), architect if needed, reviewer (opus/sol reviewer
   or orchestrator diff review); vendor-mix workers across a sprint.
5. + **Decisions**: batch owner questions once with a recommendation each; when the owner delegates, record
   each decision in the subticket's `## Decision`.
6. **Run**: one `/lean-sprint` per sprint group; group very small tickets; update the task table as each lands.
6b. **Circuit breaker** (owner, 2026-10-24): every `/goal` the skill writes or runs carries one. On a loop, a
   hard architectural or OS blocker, or no progress seen by the orchestrator, clear the goal and write a
   failure report in the epic (what was tried, why it failed, resume point).
7. + **Skip rule**: a subticket that needs new commands or unavailable resources is skipped and named, not forced.
8. + **Close**: the epic closes when every task is closed, skipped with a reason, or parked in its own ticket;
   the close reason lists each.
9. + **Evergreen and wrap**: sprint study in `docs/studies/`, ReviewLearnings entries, follow-up tickets.

## Notes

- Delegate, don't restate: lean-sprint (roles, review), issue (filing), story (study), evergreen, wrap.
- The 894 run (docs/studies/2026-10-24-two-agent-lean-sprint-epic-894-vendor-mixed.md) is the worked example.
