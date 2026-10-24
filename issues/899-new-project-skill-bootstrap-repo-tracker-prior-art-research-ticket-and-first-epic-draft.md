# 899 — new-project skill: bootstrap repo, tracker, prior-art research ticket and first epic draft

**Status**: Closed — skill docs/commands/new-project.md + config entry shipped; scratch dry run of local steps clean, found tracker bug 901 (workaround in step 4); live run parked in 902
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
3. `git init` on the default branch; + `.gitignore` for the stack, a short README with a Why section;
   LICENSE defaults to AGPL-3.0-or-later (owner default) unless the interview says otherwise.
4. `harnez init` with flags chosen per stack. Owner default (Go CLI):
   `harnez init --quota-1 --variant lite --docs make,golang,bash,manpages`. Other stacks swap the `--docs`
   list (e.g. rust, zig, cpp, website, containerfile) and drop `manpages` when there is no CLI; the skill keeps
   a small stack → flags table and states the chosen command before running it. + Confirm `.harnez/repo.yaml`
   detection (tracker, docs, studies).
5. **Make task runner**: a Makefile (docs/Make.md conventions, self-documenting `make help`) with at least
   `make build`, `make install` and `make reuse` (REUSE/SPDX license check); each may start as a no-op that
   says so, and gets real recipes as the stack lands.
5b. + First commit (conventional message) before anything else, so later steps diff cleanly.
6. **Create repos** (outward-facing: confirm first) with the forge CLIs: `fj repo create <name> --private`
   (Codeberg/Forgejo) and `gh repo create <name> --private` (GitHub). The agent picks the forges from what it
   knows about the user (this owner: Codeberg canonical + GitHub dev mirror, docs/Git.md); private unless the
   interview says public. Add remotes; push only when the owner says so.
7. **First ticket = bootstrap epic** (via `/epic`, 900): its tasks are steps 8–10 below (research ticket,
   epic refinement into work subtickets), plus the project-goal record from the interview, + a "walking
   skeleton" task (build, one test, `make test` green), + a release/CI task only if wanted.
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

- Every `/goal` (bootstrap epic, research ticket) carries the circuit breaker from 900 step 6b.
- Delegate, don't restate: HarnezInit (re-sync), story (study shape), epic (900), lean-sprint (execution).
- `uman` auto-discovers repos under `~/projects`; no `.uman.toml` entry is needed (website projects excepted).
