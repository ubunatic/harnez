# 1189 — Add opt-in AGit doc via init --docs agit

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: 1062, 1188, docs/lang/Git.md

---

## Goal

/goal A project opts in with `harnez init --docs agit`, and its agents then know the repo uses AGit:
"push" or "push to main" means `git push origin HEAD:refs/for/<default>/<topic>`, no branch is pushed,
a merged topic's next push opens a new PR, and `harnez pr wait` signals the merge. Done when the doc
ships as a copyable source, `init --docs agit` installs and links it, and a test covers that. Stop
and report when blocked on a user decision or denied permission; on a loop or no progress, clear the
goal and write a failure report here.

## Problem

AGit guidance lives inside `docs/lang/Git.md` (team mode) and is mixed with branch workflows. Agents in
an AGit repo keep proposing branch pushes or direct pushes to main, and the owner has to re-explain
AGit each session (see 1188's push-main item). A short opt-in doc gives a repo one explicit signal.

## Scope

- New copyable source, likely `docs/practices/AGit.md` (category to confirm against `docs/Markdown.md`
  and the Docs Layout rule), plus a lite variant if the sibling docs have one.
- Content: refspec and topic reuse, what "push"/"push to main" means, merged-topic behaviour, `pr wait`,
  never force-push, GitHub has no AGit.
- `docs/lang/Git.md` points to it instead of repeating it; avoid duplicate text.
- `init --docs agit` resolves the name and adds the AGENTS.md link line.

## Open questions

- Should `.harnez/repo.yaml` `git.mode: team` with an AGit forge imply this doc automatically?
- Overlap with 1062 (push/pull ergonomics): this ticket is docs only.
