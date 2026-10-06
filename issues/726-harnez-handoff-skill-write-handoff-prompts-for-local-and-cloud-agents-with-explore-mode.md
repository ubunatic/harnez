# 726 — harnez-handoff skill: write handoff prompts for local and cloud agents, with explore mode

**Status**: Closed — M1 spec/schema/loader bb3caf39, M2 skill 0727a302, M4 spec rendered into SKILL.md at apply 29253bf2, M5 missing-ticket and push-first rules 2478d640; dry run recorded; follow-ups 727, 728, 729
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `docs/commands/` (skill sources), `docs/commands/issue.md` (handoff-brief rules), issues/617 (`/respect`, recent skill ticket)

---

## 1. Problem & Motivation

The user has spare agent capacity outside the host session, e.g. the Google Jules cloud
agent from their Google AI plan, connected to their GitHub repositories. Handing work to such
an agent today is a manual session: on 2026-10-06 a Claude host session in `~/projects`
surveyed all repos, picked three tasks for Jules and wrote ready-to-paste prompts. That
workflow should be one skill, `/harnez-handoff`, whose output is always a **prompt** for
another agent. Exploration is part of the same skill (explore mode), not a separate
`harnez-explore` skill, to avoid adding more skills.

What the 2026-10-06 session did (the reference workflow for explore mode):

1. Listed every repo under `~/projects` with its remotes, last commit, recent commit rate,
   open tickets and language. Excluded the repos the user named (harnez, loom, cati, lmcoder).
2. Checked which repos the agent can actually reach: Jules sees GitHub only. Most repos use
   Codeberg as `origin` with GitHub as a secondary remote (`github` or `mirror`). Repos whose
   GitHub copy is stale (emojig: 172 commits behind) were dropped.
3. Measured local activity (commits in the last 3 and 14 days) to avoid merge conflicts with
   the user's own sessions; very active repos were dropped (ubunatic.com, loom-games).
4. Read candidate open tickets and kept those that are self-contained, verifiable without
   containers, GPUs or a screen, and confined to a separate folder or files.
5. Wrote one prompt per task: repo, ticket, the fix direction, the exact verify command
   (`make check-fast`, `make test`, `make validate-spec`) and which files to touch.
   Told the user that the PR lands on GitHub and must be fetched from the `github` remote.

## 2. Technical Specification / Findings

**Invocation.** Free prose after the skill name; no CLI flags. Expected forms:

- `/harnez-handoff 024 cloud Jules` — ticket 024, cloud profile, Jules
- `/harnez-handoff this work to Jules` / `... to a cloud agent` — current session's work
- `/harnez-handoff cloud`, `/harnez-handoff Jules`, `/harnez-handoff Copilot`
- `/harnez-handoff` (no arguments) — ask the user once for target work and profile/agent
- `/harnez-handoff explore for Jules`, `explore cloud`, `explore local`,
  `explore for local Claude` — pick one or more valuable units of work, then write prompts

**Profiles** (the user must choose one; a named agent may imply it):

- `local` — a regular local agent with the same filesystem, tools, sibling repos and
  installed binaries as the host session. The prompt can reference absolute paths, local
  tools (`harnez`, `uman`, `make install`) and other repos.
- `cloud` — works isolated in a single repository checkout. Tools only via public downloads
  (e.g. `go install ubunatic.com/harnez/cmd/harnez@latest`). Checking out other git repos is
  possible but not guaranteed. No local paths, no host-only state. The prompt must be
  self-contained and include the verify command that runs in that sandbox.

**Named agents.** Optional. The host applies what it knows about the agent when writing the
prompt. Known examples: local — Claude, Codex, Gemini; cloud — Google Jules, GitHub Copilot
coding agent. Jules facts given by the user: runs on large Intel (non-AMD) machines, can build
but not run containers, can install public packages, works on GitHub repos and opens
branches/PRs. Keep agent-specific facts in a spec or doc section, not scattered in prose.

**Explore mode** selection rules (from the reference session):

- The target repo must be reachable by the agent (cloud: hosted where the agent is connected,
  and that copy is current).
- Avoid repos or files the user's local sessions are actively changing. Tasks in a separate
  folder are fine even in busy repos: new docs, new tests, filing tickets.
- Each task must be verifiable inside the agent's profile (no containers, GPU, display or
  host-only tools for cloud).
- Prefer high-value tickets with a clear fix direction; state the merge-conflict risk.
- Ticket filing by the remote agent can collide with locally reserved numbers; the prompt
  should say how to handle it (e.g. renumber on merge).

**Output.** The prompt(s), ready to paste, plus for cloud: how the result comes back
(branch/PR on which remote, how to fetch and merge it locally).

## 3. Implementation & Verification Plan

/goal Add `/harnez-handoff` as a harnez-managed skill with `local` and `cloud` profiles,
optional named agent, and explore mode, installed like the other skills in
`docs/commands/`; done when the example invocations above each produce a correct prompt in a
dry run, or stop and report when blocked on a user decision or denied permission.

Before starting, re-check the current skill packaging in `docs/commands/` and recent commits,
since conventions may have changed since this ticket was filed.

Decisions (user, 2026-10-06):

- Agent facts (profiles, named agents and their capabilities) live in a `spec/` YAML with a
  JSON schema, per `docs/Spec.md`.
- Explore scope: if the working directory is a git repo, scan that repo. Otherwise scan the
  current repos, i.e. the git repos under the working directory (in `~/projects` that is
  the `uman` workspace; see `uman info`).

## Sprint Notes (lean sprint, host review)

Plan approved 2026-10-06 (M1 spec/schema/loader, M2 skill + registration, M3 dry run).

Pre-Work / Required Refinements for M1–M3:

- Copilot facts: add only GitHub-documented basics, marked unconfirmed in a YAML comment.
- Prime/agy (no native skill copy): fall back to `harnez read` on the spec in the harnez repo;
  record the `harnez skill show` resource gap here, do not fix it in this ticket.
- Explore mode must name the concrete checks from the reference session: the agent-side remote's
  freshness (`git rev-list --count <remote>/<branch>..HEAD`), local commit rate (last 3 and 14
  days), and the user's excluded repos.
- "this work" means the host session's current work: the prompt must carry its context
  (files, commits, decisions) because the receiving agent has no memory of the session.

### M1/M2 delivered; M3 cancelled — redirect (user, 2026-10-06)

M1 (spec, schema, loader) `bb3caf39` and M2 (skill + registration) `0727a302` delivered. The M2
design ships `spec/handoff.yaml` as a runtime resource the skill must read. The user rejected
that: **the spec drives what is in the skill document; the skill must not require agents to look
anything up in a spec.** M3 (dry run) was stopped before it started.

M4 Pre-Work / Required Refinements (do before re-running the M3 dry run):

- Generate the agent/profile facts section of the installed SKILL.md from `spec/handoff.yaml`
  at build or apply time (template or generated block), so the installed skill is self-contained.
  Reuse an existing harnez mechanism for spec-rendered docs if one exists; look before adding one.
- Remove the runtime resource (`config.yaml` `resources:` entry for `handoff.yaml`) and the
  `harnez read` fallback for agents without a native skill copy.
- Tests: the rendered skill contains every agent and profile from the spec; a spec change changes
  the rendered skill (no stale hand-copied facts in `docs/commands/HarnezHandoff.md`); the
  installed skill dir has no `handoff.yaml`.
- Then run M3 (dry run of the example invocations) against the installed skill.

### M4 delivered; M3 dry run (2026-10-06)

M4 `29253bf2`: the skill source holds a `<!-- harnez:render handoff-agents -->` line, which
`genSkillContent` replaces at apply time with markdown rendered from `spec/handoff.yaml`
(`internal/handoff/render.go`, `internal/claude/skillrender.go`). The `handoff.yaml` resource
and the `harnez read` fallback are gone. Tests: the installed skill contains every spec value,
a changed spec changes the rendered skill, the source has no hand-copied facts, and no
`handoff.yaml` is installed. Full suite: only the 3 failures that were already there before
this work (`TestAgentStartDefaultModelLine`,
`TestFlash38EscalationGuidanceAndLeanSprintDeveloperPreference`,
`TestEmbeddedAgentSpecLoadsAndResolves`).

Dry run against the installed `~/.claude/skills/harnez-handoff/SKILL.md` (after `harnez apply`),
working directory `~/projects/harnez` unless noted. Real repo data, remote-tracking refs not
re-fetched.

| Invocation | Result per skill text |
|---|---|
| `024 cloud Jules` | Profile cloud, agent Jules, consistent. harnez has no ticket 024 (`harnez issues show` fails), so the skill (M5) asks once: "harnez has no 024; trafficsim 024 is Blocked (depends on 019). Which work should Jules get?" It does not substitute a ticket. |
| `this work to Jules` | Cloud/Jules. The prompt carries this session's goal, commits (bb3caf39, 0727a302, 29253bf2), decisions and what is left. Reachability check (M5, after `git fetch github`): `github/main` has not diverged (0 commits not in HEAD); local `main` is 17 commits ahead, all by `ubunatic@noreply.codeberg.org` (= `user.email`). The repo is kept: the skill tells the user to run `git -C ~/projects/harnez push github main` and marks the prompt "ready after the push". Result: branch + PR on `github`, fetch with `git -C <repo> fetch github`. |
| `cloud` | No work given, so the skill asks once for the work (ticket, this work or explore). Writes for the cloud profile with no agent facts. |
| `Jules` / `Copilot` | Agent implies cloud; no work given, so the skill asks once for the work. |
| (no arguments) | Asks once for the work and the profile or agent. |
| `local Jules` | Conflict (Jules is cloud), so the skill asks once. |
| `explore for Jules` | Scope = the harnez repo (cwd is a git repo). harnez: `github` is only behind by the user's own commits (M5: kept, push first), but it had 101 commits in 3 days, so it is dropped as busy. Run from `~/projects` instead (workspace scope): has a GitHub remote, current, quiet (≤5 commits in 3 days): trafficsim, goto, spriteview. Dropped: emojig (172 behind), cati/loom/loom-games/lmcoder/ubunatic.com (very active), goha/homeserver (GitHub copy behind). Picks: trafficsim 028 (stat label, single file, `make check-fast` headless Node; low conflict risk) and goto 006 (resolvers, Go, `make check`; low risk, 4 commits/3d). |
| `explore cloud` | Same as `explore for Jules`, but without agent facts. Hosts come from the agents' `Reaches remotes on` lines (github.com only), so the result is the same. |
| `explore local` / `explore for local Claude` | No reachability filter; the busy check still drops very active repos unless the task stays in a separate folder. Prompts may use absolute paths and `make install`. |

Open points found by the dry run (1 and 2 fixed in M5 `2478d640`; 3 is issues/728):

1. Unknown or blocked ticket: the skill does not say to check that the ticket exists in scope
   and is not Blocked, or to ask once otherwise (see `024 cloud Jules`).
2. "Behind" check direction: `rev-list --count <remote>/<branch>..HEAD` counts local commits
   not yet pushed. That covers "the agent's copy is current" for local work. The skill could
   also say to push first when the count is only the user's own unpushed commits.
3. Installed harnez skills have two frontmatter blocks (the generated one plus the source's,
   which now sits in the body). `disable-model-invocation: true` therefore lives in the body
   for every skill whose source has frontmatter (e.g. harnez-init too). This existed before
   726; invocation is controlled by the debloat `skillOverrides` setting.

### M4 reviewed; M5 Pre-Work (host, 2026-10-06)

M4 (spec rendered into SKILL.md at apply) `29253bf2` reviewed: diff clean; installed Claude and
Codex skills contain all agents, no render marker, no `handoff.yaml`. The 3 failing tests also
fail on `2f4f946e` (before 726); filed as issues/729. Double frontmatter filed as issues/728.

M5 (skill text only, `docs/commands/HarnezHandoff.md`):

- Ticket not found in the target repo, or Blocked/Closed: ask the user once (name what was found,
  e.g. "harnez has no 024; trafficsim 024 is Blocked"), never pick a substitute silently.
- Freshness: if the agent's remote is behind only by the user's own unpushed commits, do not drop
  the repo; tell the user to push to that remote first (name the command) and mark the prompt as
  ready after the push. Drop only when the remote has diverged or the gap is not the user's.
- Re-run the two affected dry-run cases and update the ticket's dry-run section.
