---
kind: retrospective
date: 2026-11-09
---
# 2026-11-09 — Forge rough edges, release v0.5.7, agent shell

## Delivered (all through AGit PRs, each merge confirmed by `harnez pr wait` exit 0)

- 1198/1199 forge find and show fixes (#136–#142), argon review findings fixed in #142,
  review learnings in #143.
- Release v0.5.7 (#144).
- Tickets filed: 1200, 1201, 1202 (forge issue edit), 1203 (official model names).
- Docs: AGit "existing pull request" wording note (#146), AgentShell.md (#147).

## What worked

- AGit with one fresh topic per change: 13 PRs, no force-push, no push to main.
- Background `pr wait` as the only merge signal; no merge was claimed from chat.
- A cross-vendor one-shot review (argon:low, 6 min) found three real medium bugs that the
  host review had passed.
- Shell question settled with evidence (ps output from a hook-free session) plus the official
  env-var docs, before any recommendation.

## What did not

- Close reason for 1198 says "exit code not captured" for `make test-q1`: a weak pass claim.
  Safeguard: lean-sprint's rule to quote the final test summary line (already in the skill).
- Ticket 1202 (filed early, "fix next") is still unstarted at session end; the session drifted
  into docs and filing. Safeguard: it stays Open in the tracker as the next task.
- 35 active tickets exceed the cap of 30 (push warning); four new tickets added today.
  Proposed safeguard: a grooming pass before the next sprint.

## Open at session end

- 1202 `harnez issues edit` for forge trackers (next).
- 1203 official model names.
- Owner decision: `CLAUDE_CODE_SHELL=/usr/bin/bash` in settings (docs/AgentShell.md).
