# 1194 — /issue skill: forge-tracker path for issues.tracker none (search, create, labels, overlap, reporting)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: 1116 (forge show/find), 1118 (forge token), 1139

---

/goal The `/issue` skill and `.harnez/rules/Index.md` give a complete path for repos whose tickets
live on the forge (`issues.tracker: none`): which forge repo and tool, duplicate search, create,
labels, how to handle overlap, and what to report. Done when an agent can file a forge ticket in
taia without guessing. Stop and report when blocked on a user decision; on a loop or no progress,
clear the goal and write a failure report here.

## Problem

Field report from filing taia#217 (2026-11-09, `/tmp/issue-skill-feedback.md`):

- `Index.md` for `tracker: none` says "use the repo's own tracker" but does not name it; the agent
  inferred Codeberg `harnez/taia` and `tea` from commit messages.
- Duplicate search was a `grep` over the 100 newest titles. `harnez find issues` already reads forge
  tickets (1116) but nothing points the agent to it.
- Partial overlap with #182: the skill has no rule, and the agent posted a public comment on #182
  without asking.
- No labels were applied; the skill does not say to check the forge's labels.
- "Publish Promptly" covers commits only; nothing says how to report a forge ticket.

## Scope

- `Index.md` (`tracker: none`): print the forge repo (from origin) and the tool; point to
  `harnez find -d . issues` / `issues show` for search. Decide whether `repo.yaml` needs an explicit
  `issues.forge` / `cli` (spec + schema, not Go defaults); 1116 deliberately added no tracker value.
- Skill, forge path: search with `harnez find issues` (note 1116: no server-side body search),
  create with `tea issues create` / `gh issue create` from a body file, apply priority/category labels
  that exist on the forge (`tea labels`).
- Overlap rule: when an existing ticket partly covers the request, say so and ask before filing or
  commenting; commenting on other tickets needs the user's OK.
- Reporting: "filed as #N at <URL>; no commit"; state that Publish Promptly applies to file trackers.

## Open questions

- Does `harnez find issues` on a private forge repo work with the 1118 token in taia today (1116 notes 404 without one)?

## Decision

- No `repo.yaml` forge/cli field: the forge comes from origin and the CLI list from `spec/pr.yaml` `issue_cli` (1195). Skill `docs/commands/issue.md` gains a "Forge tracker" section; the tracker-none Index.md notice names find/show/new, the overlap rule and the report form.
- Open question (private repo with the 1118 token in taia) is not verified here.
