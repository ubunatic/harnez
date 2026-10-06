# 726 — harnez-handoff skill: write handoff prompts for local and cloud agents, with explore mode

**Status**: Open
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
