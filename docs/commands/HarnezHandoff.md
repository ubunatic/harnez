---
name: harnez-handoff
description: Write ready-to-paste handoff prompts for a local or cloud agent, optionally exploring for valuable work first
disable-model-invocation: true
---

# Harnez Handoff

The output of this skill is always one or more **prompts** for another agent, ready to paste.
Do not do the handed-off work yourself.

## 1. Read the Request

The request is free prose after `/harnez-handoff`; there are no flags. Pick out:

- **Work:** a ticket number (`024`), `this work` (the current session's work), or `explore`
  (pick valuable work first, see section 3).
- **Profile:** `local` or `cloud`.
- **Agent:** an optional agent name, matched case-insensitively against the known agents and
  their match names in section 2. A named agent implies its profile.

Ask the user **once**, in one message, when:

- nothing is given (ask for the work and the profile or agent);
- the profile and agent conflict (e.g. `local Jules`: Jules is a cloud agent);
- the agent is unknown (list the known agents with their profiles, or offer the bare profile).

A missing agent is fine: write for the profile alone. A missing profile with no agent is not:
ask.

## 2. Profiles and Known Agents

Apply the profile's rules and the agent's facts to every prompt. For cloud agents, "Reaches
remotes on" says which remotes the agent can see, and "Result" says how the work comes back.

<!-- harnez:render handoff-agents -->

## 3. Explore Mode

Scope: if the working directory is a git repo, scan that repo. Otherwise scan the git repos
under it (in a `uman` workspace, `uman info` lists them). Skip repos the user excludes in the
request. Gather per repo: remotes, language, open tickets (`harnez find -d <repo> issues -a
status:open`), and the checks below. Keep only work that passes all of them:

- **Reachable:** cloud agents need a remote whose URL matches a host the agent reaches, and
  that copy must be current: `git -C <repo> fetch <remote>`, then
  `git -C <repo> rev-list --count <remote>/<branch>..HEAD` must be 0 (or near 0 for work the
  missing commits do not touch). Drop stale repos and say so.
- **Not busy:** count local commits in the last 3 and 14 days
  (`git -C <repo> rev-list --count --since=3.days HEAD`, same with `14.days`). Drop very active
  repos, unless the task stays in a separate folder or new files (new docs, new tests, new
  tickets).
- **Verifiable in the profile:** the verify command runs where the agent runs; for cloud, no
  containers, GPU, display or host-only tools.
- **Valuable and clear:** prefer high-priority tickets with a clear fix direction and a small,
  confined set of files.

Present the picks briefly (repo, ticket, why, merge-conflict risk), then write one prompt per
pick.

## 4. Write Each Prompt

The receiving agent has no memory of this session. Brief it like a colleague who just walked in
(see the Handoff Prompt section of the `issue` skill). Each prompt contains:

- the repo (local: absolute path; cloud: repo name on the agent's host) and the ticket, or for
  `this work` the session's context in full: goal, files and commits touched, decisions made,
  what is left;
- the fix direction, the files to touch and the files to leave alone;
- the exact verify command (e.g. `make check-fast`, `make test`, `make validate-spec`);
- a `/goal` with an exit clause: `... or stop and report when blocked on a user decision`;
- for new tickets filed by a cloud agent: ticket numbers may collide with numbers reserved
  locally; tell it to use the next free number and note that it may be renumbered on merge;
- the profile rules and agent facts from section 2 that matter for this task.

## 5. Output

Print each prompt in its own fenced block, ready to paste. For cloud agents, add below each
prompt how the result comes back, from the agent's "Result" line: which remote the branch or PR
lands on (the local remote name whose URL matches the agent's host, e.g. `github`, not
`origin` when origin is on Codeberg), and how to bring it in:

```bash
git -C <repo> fetch <remote>
git -C <repo> merge <remote>/<branch>   # or review the PR first
```
