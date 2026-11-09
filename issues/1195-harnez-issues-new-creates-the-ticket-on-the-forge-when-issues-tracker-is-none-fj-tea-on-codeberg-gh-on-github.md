# 1195 — harnez issues new creates the ticket on the forge when issues.tracker is none (fj/tea on Codeberg, gh on GitHub)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1194 (/issue forge path, docs), 1116 (forge show/find), 1067 (export local → forge), 1064 (import), 857 (external providers), 1118 (forge token)

---

/goal In a repo with `issues.tracker: none` whose tickets live on a forge, `harnez issues new "<title>" -F <body>`
detects the forge from the origin remote and creates the ticket there with the right CLI (`fj` and/or `tea`
for Codeberg/Forgejo, `gh` for GitHub), printing the number and URL. Stop and report when blocked on a user
decision; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Owner request (2026-11-09). Agents filing in forge-tracked repos such as taia guess the forge and tool
(taia#217, see 1194). `harnez issues show/find` already reads forge tickets (1116), but creating one
still means hand-built `tea`/`gh` calls. 1067 exports existing local tickets; this ticket is the
create path for repos with no local tracker.

## Want

- Detect the forge from the origin URL (reuse the 1116 / `spec/pr.yaml` forge detection, not new Go defaults).
- Pick the installed CLI: Codeberg/Forgejo `fj` or `tea` (order and which is preferred: open), GitHub `gh`;
  clear error naming the missing tool otherwise.
- Title, body from a file, optional labels that exist on the forge; no commit; print "filed as #N at <URL>".
- Local `issues/` mode is unchanged; a dry run shows the command without creating anything.
- 1194's skill text then points to this command instead of raw `tea`/`gh`.

## Open questions

- `fj` vs `tea` preference, and whether to call the forge API directly (1118 token) instead of a CLI.
- Duplicate check: the forge-side similar search that local `issues new` does (1116 has no body search).
- Is an explicit `repo.yaml` forge field needed (1194 asks the same)?

## Decision

- Trigger: `issues new` takes the forge path only with an explicit signal (`-F`/`--label`, or `issues.tracker: none` in repo.yaml) and no local `issues/` dir; a fresh repo without one still adopts a local tracker (853). `--remote`, `--number` and `CLAUDE_CODE_REMOTE` keep their paths (argon:low review, high finding).
- CLI order lives in `spec/pr.yaml` `issue_cli` (schema enum gh/tea/fj): Codeberg `tea` then `fj`, GitHub `gh`. `fj` has no create label flag, so a labelled request skips it.
- No API create with the 1118 token and no forge-side duplicate check in this ticket; the skill says to search with `harnez find issues` first.
- `ParseCreated` strips the title before reading `#N`, so "Follow-up to #182" is not read as the new number. Unverified: real `tea`/`fj` create output (no live ticket was filed); a missing number errors with "check the forge before retrying".
