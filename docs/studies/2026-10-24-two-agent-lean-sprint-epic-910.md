# Epic 910 — repo setup, tracker, tests and agent guidance (2026-10-24)

Host: Claude Opus (orchestrator). Owner delegated decisions with `/goal complete the epic`.

## Run

| Group | Ticket | Developer | Review | Result |
|---|---|---|---|---|
| fanout | all | haiku:low searcher, 57 s | — | 2 of 7 premises stale (353 done, 907 no copy fixtures) |
| — | 353 | — | host | closed, already delivered by the session-start hook |
| A | 898, 867 | haiku:high | host diff | both closed in one run |
| B | 907 | luna:high | host diff | audit found no copy fixtures; doc rules replaced |
| C | 864 | sol:med, 3m45s | host diff | release preflights and pushes every `git remote` |
| D | 897 | sol:med, 11 min + M3 | opus:low | ship verdict; 5 fixes in M3; daemon persistence parked in 913 |
| E | 855 | sol:med, 6 min + M3 | opus:low + host pty | blocker found: wizard opt-out undone by later `-y` init; fixed in M3 |

## Findings

- **Fanout pays off on old tickets.** One cheap searcher pass closed one ticket outright and shrank another to
  a doc change before any developer started.
- **The reviewer catches what the developer flags as a "limitation".** In 855 the developer reported
  "a later run can re-enable a disabled tracker" as an open point; the reviewer classified it as a blocker
  against the goal. Durable safeguard: lean-sprint §5 already requires comparing delivery with the `/goal`; the
  host should treat every developer "open point" as a goal check input (added to `docs/ReviewLearnings.md`).
- **Post-run test edits.** The 855 M3 developer changed three test expectations after its one allowed full
  run and closed the ticket "unverified"; the host reran the touched packages (green) and amended the close
  reason. This is the 896 rule working as designed.
- **New 867 gate in use.** Ticket-only closes now need `--no-code` (this epic's own close).

## Follow-ups

- 913: installed `harnez server` cannot index (flags not persisted by `server install`).
- 912 (filed during the run, not in the epic): swap in the usage Load boxes.
