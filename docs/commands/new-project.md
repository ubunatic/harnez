# Bootstrap a New Project

Take an idea to a repo with tracker, a bootstrap epic and a prior-art research ticket.
You, the agent that received this invocation, run every step yourself.

Invocation: `/new-project "<idea>"` or `/new-project <name> "<idea>"`.

Delegate, don't restate: `/epic` (epic draft, fanout, refinement), `/issue` (filing),
`/story` (study shape), `/lean-sprint` (execution). Stop and ask before any outward step
(creating remote repos, pushing) and when the answers leave the purpose unclear.

## 1. Interview (ask once, batched)

Ask in one message, each with a default:

- Purpose and users; name.
- Language/stack; shape (CLI, library, website, app).
- Non-goals; hard constraints (offline, no deps, platforms).
- License (default AGPL-3.0-or-later); public or private (default private).
- Remotes: pick the forges from what you know about the user (e.g. Codeberg canonical +
  GitHub mirror); ask when you know nothing.

Record the answers in the bootstrap epic (step 7), not only in chat.

## 2. Name Check

The name is free locally (`~/projects/<name>` absent; `uman info` when available) and, if
public, on the chosen forges.

## 3. Repository

- `git init` on the default branch.
- `.gitignore` for the stack, a short README with a Why section, LICENSE (AGPL-3.0-or-later
  unless the interview says otherwise).

## 4. Harnez Init

Choose flags per stack and state the command before running it:

| Stack | Command |
|---|---|
| Go CLI (default) | `harnez init --quota-1 --variant lite --docs make,golang,bash,manpages` |
| Go library | `harnez init --quota-1 --variant lite --docs make,golang,bash` |
| Rust | `harnez init --quota-1 --variant lite --docs make,rust,bash` |
| Zig | `harnez init --quota-1 --variant lite --docs make,zig,bash` |
| C++ | `harnez init --quota-1 --variant lite --docs make,cpp,bash` |
| Website | `harnez init --variant lite --docs make,bash,website` |

Add `containerfile` when the project ships images; drop `manpages` without a CLI.
Check `harnez docs` for the current names.

Create `docs/studies/` (with a `.gitkeep`) before `harnez init` so studies are detected.
The tracker appears with the first ticket: after step 8 files the epic, check
`.harnez/repo.yaml`; if it still says `issues.tracker: none`, delete that file and rerun
`harnez init -d .` (bug 901), then expect `tracker: harnez`.

## 5. Make Task Runner

A Makefile following the Make doc (self-documenting `make help`) with at least `make build`,
`make install` and `make reuse` (REUSE/SPDX license check). Each may start as a no-op that says
so; real recipes come with the stack.

## 6. First Commit

Commit everything so far (`chore: bootstrap <name>`), before any further step.

## 7. Remote Repos (confirm first)

After the user confirms:

- Codeberg/Forgejo: `fj repo create <name> --private`
- GitHub: `gh repo create <name> --private`

Drop `--private` only when the interview said public. Add the remotes; push only when asked.

## 8. Bootstrap Epic (first ticket)

File it with `/epic "<idea>"`. Its tasks:

- Project-goal record (interview answers).
- Prior-art research ticket (step 9).
- Epic refinement into work subtickets (after the research lands).
- Walking skeleton: build, one test, `make test` green.
- Release/CI only if wanted.

The epic and every ticket it files carry a `/goal` with exit clause and circuit breaker:
on a loop, a hard architectural or OS blocker, or no progress, clear the goal and write a
failure report (what was tried, why it failed, resume point).

## 9. Prior-Art Research Ticket

File it with `/issue`: web fanout per `/epic` step 2 (existing tools, libraries to reuse,
standards, pitfalls) → aggregate → study (studies dir when present, otherwise the ticket).
Each finding cites its source and rates maintenance, size and license fit.

## 10. Refine After Research

When the study lands, run `/epic <n>` to turn the tasks into subtickets with acceptance, write
set, developer tier (haiku vs. sol) and reviewer (opus/sol vs. orchestrator only).

## 11. Handoff

Report the repo path, remotes (if any), ticket numbers and the next command (`/epic <n>`).
