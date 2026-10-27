# 983 — spec: model picks per task class (standard, complex, mechanical, easy, searcher, architect) as the only source; skills read it

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: 942 (epic: model catalog and user model policy), 974 (Terra onboarded), 973 (luna:med default, Haiku tiers), 893 (Haiku as searcher)

---

/goal Which model to use for which kind of task is defined once in `spec/agent.yaml` (with schema), harnez prints and
resolves it, and the sprint skills and practice docs refer to it instead of naming models in prose; or stop and report
when blocked on an owner decision or a denied permission. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Owner, 2026-10-27: "I want that everything is well-specced. Eventually, we need to get rid of the prose, and the model
configuration should be the only source of truth." Terra (974) should be the standard developer and Sol the model for
complicated tasks.

Today the spec has per-model `roles` (developer, advisor, reviewer, mechanical; a recommendation, not enforced) and
free-text `use` / `use_med` lines, but no task-class picks. The picks live as prose, repeated and drifting, in
`docs/commands/lean-sprint.md`, `lean-sprint-v1.md`, `epic.md`, `HarnezStatus.md`, `docs/practices/AgenticLoop.md`
(full and lite), `docs/practices/ModelRoles.md`, and the `use_med` line of `codex:luna` ("cheap default developer").
Every model change (973, 974) needed edits in all of them.

## 2. Proposed picks (owner direction plus current guidance)

| Task class | Model | Source |
|---|---|---|
| standard developer | `terra:low` | owner, 2026-10-27 |
| complex developer (judgment, orchestration, hard compiler failures) | `sol:low` | owner, 2026-10-27 |
| mechanical / well-specified cheap work | `luna:med` | host reading; owner to confirm |
| easy task | `haiku:med` | 973 |
| very simple, repetitive task | `haiku:low` | 973 |
| searcher (read-only discovery, web) | `luna:low`, `haiku:low` | lean-sprint §1, 893 |
| architect | `opus:low`, `opus:med`, `sol:low`, `sol:med`, `argon:low` | lean-sprint §1.5 |
| reviewer | open | owner decision |

## 3. Implementation & Verification Plan

1. Design (open): spec shape (e.g. a `picks:` map from task class to an ordered model list), and how skills use it
   without naming models. One option: `--model` accepts a task-class reference (e.g. `@standard`), so skills say
   `harnez agent start --model @standard` and the spec decides. Per the epic rule, the schema ships with its first
   reader and a visible result.
2. Schema entry, loader and validation (unknown model or class fails), `harnez agent models` shows the picks.
3. Replace the model names in the skills and practice docs listed in section 1 with the class reference; keep
   full and lite sources and root copies in sync (`harnez init -d .`).
4. Verify: tests for resolution and validation; `harnez agent start --model @standard` resolves to `terra:low`;
   `rg 'luna:med|sol:low|haiku:high' docs/commands docs/practices` shows no task-class picks left in prose;
   `make test-q1`; `make install`.
