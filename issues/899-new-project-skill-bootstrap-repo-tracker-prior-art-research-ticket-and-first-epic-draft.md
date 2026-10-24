# 899 — new-project skill: bootstrap repo, tracker, prior-art research ticket and first epic draft

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 900 (epic skill), docs/commands/HarnezInit.md (re-sync of an initialized repo), docs/commands/story.md,
docs/commands/roadmap.md, docs/commands/jam.md, docs/practices/AgenticLoop.md, docs/Git.md (Remotes)

---

/goal A harnez-managed `new-project` skill (docs/commands/new-project.md + config.yaml `skills:` entry) takes an
agent from an idea to a repo with tracker, first tickets, a prior-art research ticket and a draft epic that
`/epic` (900) refines; verified by one dry run in a scratch dir under `~/projects` that ends with a clean
`git status` and the expected tickets; stop and ask before any outward step (creating remote repos, pushing)
or when the owner's answers leave the project's purpose unclear.

## Owner request (2026-10-24)

Run git init and harnez init, create repos, create first tickets, ask the user questions, file a ticket for a
research fanout + aggregate + study on prior art, and create a first epic draft. The epic is refined after the
fanout so it gets concrete work subtickets, each well defined, with a proposed developer model level (haiku vs
sol) and reviewer (opus/sol vs orchestrator only).

## Steps (owner's list, plus host-added details marked +)

1. **Interview** (ask once, batched): purpose and users, name, language/stack, license, + CLI/library/
   website/app shape, + non-goals, + hard constraints (offline, no deps, platforms), + public or private,
   + which remotes. Record answers in the first ticket, not only in chat.
2. + **Name check**: the name is free locally (`~/projects/<name>`, `uman info`) and, if public, on the forges.
3. `git init` on the default branch; + `.gitignore` for the stack, LICENSE, a short README with a Why section.
4. `harnez init -d .`; + confirm `.harnez/repo.yaml` detection (tracker, docs, studies) and a `make help`
   that works; + `make install` only if the Makefile provides it.
5. + First commit (conventional message) before anything else, so later steps diff cleanly.
6. **Create repos** (outward-facing: confirm first): Codeberg canonical, GitHub dev mirror per docs/Git.md;
   add remotes; push only when the owner says so.
7. **First tickets**: a project-goal ticket (from the interview), + a "walking skeleton" ticket (build, one
   test, `make test` green), + a release/CI ticket only if wanted.
8. **Prior-art research ticket**: web-search fanout (several cheap searchers, distinct angles: existing tools,
   libraries to reuse, standards/specs, known pitfalls) → aggregate → study in `docs/studies/`. + Each finding
   cites its source; + rate maintenance (last release, activity), size/bloat, license fit.
9. **Epic draft** via `/epic` (900): goal, out of scope, open questions; tasks stay placeholders until the
   research study lands.
10. + **Refine after research**: update the epic so every task is a ticket with acceptance, write set, a
    proposed developer tier (haiku for well-specified, machine-checkable work; sol for judgment) and reviewer
    (opus/sol reviewer for risky work; orchestrator diff review for small work).
11. + **Handoff**: report the repo path, remotes (if any), ticket numbers and the next command (`/epic <n>`).

## Notes

- Delegate, don't restate: HarnezInit (re-sync), story (study shape), epic (900), lean-sprint (execution).
- `uman` auto-discovers repos under `~/projects`; no `.uman.toml` entry is needed (website projects excepted).
