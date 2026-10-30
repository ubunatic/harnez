# 1039 — Per-repo git workflow mode: solo (default), team, protected main

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Enhancement
**Related**: [[864]] (session push consent, release pushes per remote), [[836]], docs/Git.md, docs/GoRelease.md

---

## Goal

/goal Make the git workflow a single per-repo setting in `.harnez/repo.yaml`. The default stays `solo`;
`team` and protected-main modes make the agent rules, sprint skills and harnez's own commits and pushes
follow the branch + PR flow. Verify each mode in a scratch repo. Stop and report when blocked on a user
decision or a denied permission. On a loop, a hard blocker, or no progress, clear the goal and write a
failure report here.

## 1. Problem & Motivation

harnez assumes solo work: `docs/Git.md` says to work on the default branch, and `harnez issues new|close`
and the sprint skills commit straight to `main`. harnez itself is now worked on as a team.
`ubunatic/harnez` is the canonical remote (it is `origin` again; the old `flashcoder` copy is a remote
that only the trigger or the owner pushes), and its `main` is protected. Every change must go through a branch
and a PR (e.g. PR #14 for 1036/1037). Today that is handled by hand on every session, and nothing in the repo
records it, so the next agent may commit tickets to `main` and try to push.

## 2. Proposal

A per-repo key, e.g. `git.mode` in `.harnez/repo.yaml` (name open), with `harnez init` rendering the matching rules:

| Mode | Commits | Publishing |
|---|---|---|
| `solo` (default, today) | on `main` | push `main` when asked |
| `team` | on a topic branch per ticket/sprint | push the branch, open a PR (`tea`/`gh` per forge) |
| protected `main` | as team; never commit to local `main` | as team; never push `main` or force-push |

- Setting it must be one command or one line, e.g. `harnez init --git-mode team`.
- Rendered guidance (docs/Git.md copy, rules) and the sprint/issue skills read the mode instead of assuming `main`.
- `harnez issues new|close` and other auto-committing commands: in team/protected modes, refuse on `main`
  or commit to the current topic branch.
- `harnez release`: check that tag and branch pushes work with a protected `main`.

## 3. Open Questions

- Is protected `main` its own mode, or a flag on `team` (`git.protected_main: true`)? Could it be read
  from the forge API instead?
- Branch naming (`issue/<n>-<slug>`, `sprint/<tickets>`) and who opens the PR: host or developer.
- Where ticket-only commits go in team mode: their own PR, or riding along with the next code PR.

## 4. Verification

- Scratch repo per mode: `harnez init` renders the matching rules, and the issue commands commit where the mode says.
- In this repo, `team` plus protected `main` is set, and a full lean sprint ends in a PR with no push to `main`.
