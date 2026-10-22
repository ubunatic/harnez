# 864 — push consent lasts the session, and a push always syncs Codeberg and GitHub

**Status**: Open
**Priority**: P2 (Medium) — agents ask "push where?" after the user already said push, and remotes drift
**Severity**: Minor
**Category**: Feature
**Related**: 863 (harnez release pushes only origin), docs/lang/Git.md (Remotes)

---

/goal Agents stop re-asking about pushes once the user has confirmed them in a session, and every push keeps
Codeberg and GitHub `main` (and tags) equal, done by a tool rather than by agent memory. Verified by the
updated Git doc (source + root copy) and a test or live run of the sync step, or stop and report on a
design choice.

## Findings (2026-10-22)

- `docs/lang/Git.md` says "Do not push unless asked" and "Keep both `main` branches equal ... goes to both
  remotes", but not that one confirmation covers the session, nor that "push" means both remotes.
- This session: the user said "push" several times; the host still asked "also push to GitHub?" before
  the v0.2.0 release, then pushed GitHub without an answer.
- `harnez release` pushes branch and tag only to `origin` (Codeberg); GitHub got the tag by hand.

## Way forward

1. Git doc (source `docs/lang/Git.md`, then sync root copy): once the user confirmed pushing in a session,
   push later finished work without asking again; "push" means `main` and tags to both dev remotes; ask
   only for force pushes, new remote branches, or a remote that diverged.
2. Programmatic sync: one command pushes `main` and tags to every configured dev remote (fast-forward only,
   stop on divergence). Check `uman push` first; else a `harnez` step reused by `harnez release`.
   Remote names from config/spec, not Go defaults.
