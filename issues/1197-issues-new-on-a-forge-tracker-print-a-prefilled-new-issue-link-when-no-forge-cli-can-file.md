# 1197 — issues new on a forge tracker: print a prefilled new-issue link when no forge CLI can file

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1195 (forge create via tea/fj/gh), 1196 (forge duplicate gate), 1067 (issues export)

---

/goal When `harnez issues new -F` on a forge tracker cannot file (no CLI from `issue_cli` installed, or the CLI
fails for lack of a login), it prints a new-issue link with title and body prefilled for a human to submit, and
exits non-zero so no agent reports the ticket as filed. Stop and report when blocked on a user decision; on a
loop or no progress, clear the goal and write a failure report here.

## Problem

Owner question (2026-11-09): can AGit help when fj/tea are not logged in? No: AGit opens pull requests only, it
cannot create issues. Filing needs a write login (tea/fj/gh) or a write token; `CODEBERG_TOKEN` is usually
read-only. Today `issues new -F` then ends with "no forge CLI found…" or the CLI's auth error, and the drafted
body is left with the agent.

## Want

- Fallback output: `not filed: <reason>; open <URL> to file it` plus the link, e.g.
  `https://<host>/<owner>/<repo>/issues/new?title=…&body=…` (URL-encoded); `--json` carries the URL.
- Also offered on `--dry-run` alongside the planned CLI command.
- Body too long for a URL: link with title only, and say the body is in `<body-file>`.
- Labels: GitHub accepts `labels=a,b`; on Forgejo only if a canary shows a name-based parameter works.
- Agent report form (skill /issue, Index.md notice): "not filed, link handed to the user".

## Notes

- Unverified: Forgejo/Gitea honouring `title` and `body` query parameters on `/issues/new`, and the URL length
  each forge and browser accepts. Canary both on codeberg.org before building (docs/Canary.md).
- Telling "not logged in" from other CLI failures may need per-CLI stderr matching; a simple first cut offers the
  link on any CLI failure.
- Field report (taia, 2026-11-09): first real filing worked (taia#223 via tea 0.16.0). `--label bug` on a repo
  with no labels passes `--dry-run` unchecked; consider a label pre-check (`tea labels list` / `gh label list`)
  in the same error path, so a missing label fails with a clear message before filing.
