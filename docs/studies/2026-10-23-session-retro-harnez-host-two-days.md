# Session retro: harnez host session, 2026-10-21 to 2026-10-23

Retro of one Claude Code host session (Opus) in harnez, written from a cheap extract (issue 878).
Sprint-level detail is in the per-sprint studies of these days (`2026-10-2[123]-two-agent-lean-sprint-*`,
`2026-10-21-ticket-hygiene-*`, `2026-10-22-generic-init-repo-yaml-story.md`); this file does not repeat them.

## Context

46 hours of wall time (17:36 on the 21st to 15:06 on the 23rd), 7 compactions, 18.3 MB transcript. Starting
point: ticket hygiene (active list capped at 20). Then lean sprints on generated-file merges (847), repo config
(`.harnez/repo.yaml`, 854/856/860), the v0.2.0 release, usage freshness and heartbeat (714), agy quota false
alarms (868), agy status-line drops (710) and "usage never hangs" (873).

## Summary

The host ran lean sprints back to back with luna/sol developers and stayed out of the code, except for one
small direct fix (the 714 startup deadlock, `b47b1263`). Tickets closed in the window include 710, 714, 847,
850, 853, 854, 856, 860-863, 868 and 873. Several closes were reopened first because the host's own run of the
real flow contradicted a green developer report.

## What worked

- Running the installed binary and the real flow after each milestone: it caught 847 M2/M3 (driver not
  installed, merge rows lost), 850 (closed with acceptance unmet) and the 714 hang.
- Short owner decisions ("go", "file both", "push + sprint 856") turned straight into sprints; the owner's
  messages are mostly one-liners.
- Owner-run live probes (agy status line, restarting services) settled questions code reading could not.

## Post-mortem

- **History rewrite by a worker (bug-hunt, 22nd):** dev-b4 autosquashed pushed commits; main diverged and a
  pushed ticket looked untracked. Recovered by rebase onto origin; rule "never amend/rebase" now in the
  lean-sprint checklist and ReviewLearnings.
- **Developer closes without acceptance:** 850 closed at 0.594 above the cutoff; now "close only after the
  acceptance check, reason states the evidence" in the checklist.
- **Rendered output edited instead of the source (843 M3):** init reverted it; ReviewLearnings entry.
- **Host rule breaks:** two heredoc appends against the owner's edit rule; one GitHub push before the owner
  answered (disclosed); a wrong commit hash reported once.
- **Handoff errors:** told sol:med the tree was clean while dev-714 had left uncommitted M4 work; a forgotten
  background job the owner had to point out (led to 870).
- **Wrong claims:** "no harnez code renders % left" (710; it is `spec/statusline.yaml`); the agy "quota
  reached" was a replay of an old error (868).
- **Circular readiness (714 M3):** `harnez usage` hung 30s after every server restart because readiness
  waited on a fetch that waited on a client heartbeat. Tests always started with data. Fixed in `b47b1263`,
  generalised in 873; the lean-sprint skill now says "test services the first minute after start".
- **Open at the end:** `usage --minimal --offline` still takes 8.2s with 1% CPU. The owner's own `time` run
  points at the remote-load SSH call, not the store read the host had blamed (874).

## Quality and invariants

| Area | Assessment |
|---|---|
| Architecture | Client never waits on the server (873); readiness independent of clients (714). |
| Idempotency | `harnez init` re-run checks caught reverted edits; required clean `git status` after init. |
| Compatibility | Counter rename `passive_dropped` to `passive_no_quota` (710) changed a JSON key. |
| Tests | Built-binary tests per server state added (873); several earlier sprints had green suites with wrong live output. |

## Efficiency

About 200 commits on main in the window, shared with other sessions (not separable from this extract). Most
host time went into milestone review and reruns of the real flow; developer turns were dominated by repeated
`make test-q1` runs (873 M1: six runs in 22 minutes, one blocked by another session's uncommitted files).

## Learnings

Already promoted: lean-sprint checklist items (close evidence, no rewrites, first-minute service tests),
ReviewLearnings (circular readiness, rendered files, shared history). New proposal for the owner: measure
`user+sys` against wall time before blaming a slow path; a 1% CPU run is waiting, not computing.

## Method and gaps

Input: 7 summaries (77 KB) of which only the "Errors and fixes" sections were read (~4k tokens), 91 owner
messages (20 KB, read cut to 300 chars per message, ~4k tokens), `git log`, the studies list. Raw transcript
not opened. Gaps: the first owner message comes 2h15m after the session start, so the starting slash command
(e.g. `/lean-sprint ...`) was filtered out; long owner messages were cut by the reader; commit and close counts
include other sessions; voice-input duplicates ("hot"/"hard") appear as separate messages; no per-ticket
timings.
