---
kind: sprint
date: 2026-11-06
tickets: [959, 971, 1012, 1076, 1081, 1082, 1086, 1102, 1107, 1137, 1138, 1139, 1140-1170, 1171, 1172, 1173]
---
# Jules backlog cleanup, 2026-11-06 (living study)

Updated as the waves run. Host: Claude Opus 5.5 in decide mode. Worker: Google Jules (one-shot,
reads GitHub `main`). Jules stops accepting new work on 2026-11-11.

## Context

- Goals (owner): (1) close, merge or complete as many tickets as possible for a clean backlog;
  (2) make harnez a good companion for taia (codeberg.org/harnez/taia), or a Go AI library taia can
  take code from later. Owner sets taia priority; Jules cannot see taia.
- Rules (owner): at most 10 Jules runs at once; finish each wave (drop stuck runs) before the next;
  ask Jules to propose priorities with reasons.
- Start: 31 `Open` and 330 `Backlog` tickets. 12 earlier batches (1123-1135, except 1129) had
  covered 70 of the backlog tickets.

## Outcome

### Setup

- `~/.jules/config.json` was rejected as "not valid JSON": `github_projects` held strings, the code
  wants `{"repo": ...}` objects (`internal/subagent/jules.go:86`). Fixed by hand; the message is a
  harnez bug (filed 1173).
- Codeberg token imported (`harnez forge token import --check harnez/taia`); `harnez find -d ../taia
  issues` now reads taia's 27 open issues. Side effect: the open-PR check on public
  `ubunatic/harnez` now returns 403 (filed 1138).

### Wave 0: single-ticket runs on `80429ccd`

| Ticket | Kind | Result | Refs held |
| --- | --- | --- | --- |
| 1082 | audit | still valid | 5/5 |
| 1012 | audit | still valid; starting points added | 6/6 |
| 1107, 1076, 1102 | audit | still valid, repeat of 11-03/04 audit | 1/1, 5/5, 5/5 |
| 971, 959 | audit | partly done, repeat | 3/3, 4/5 |
| 1086 | implement | PR #98: fallback good, name freeing unsafe | tests pass, new test fails without fix |
| 1137 | implement | failed: COMPLETED, no PR, no message (19m) | - |
| 1081 | audit | failed: COMPLETED, no PR, no message (17.5m) | - |

29 of 30 checked references held (97%). New information came from 2 of 10 runs.

### Batch waves

Wrappers 1140-1170 group the 260 remaining backlog tickets by title (31 batches of 5-10; four
small topics folded into 1168). One run per wrapper; prompt = skill audit prompt extended with a
per-ticket verdict, proposed priority and reason, merges and a top five.

| Wave | Wrappers | State |
| --- | --- | --- |
| 1 | 1140-1149 | running, started 2026-11-06 00:45 on `9f7dcf78` |
| 2 | 1150-1159 | pending |
| 3 | 1160-1169 | pending |
| 4 | 1170 | pending |

## What Went Wrong

- **Missed the backlog.** The first scan used `status:open` and reported 29 tickets; the backlog
  had 330 more under status `Backlog`. Caught by the owner. Fix: count by status first
  (`grep '^\*\*Status'`).
- **Repeat audits.** 5 of 8 delivered audits repeated an audit from 2-3 days earlier on unchanged
  code. My pre-check grepped only `## Audit` headings and missed 1102's prose note. Fix for later
  waves: drop tickets with any earlier Jules audit text; skill change filed (1171).
- **Silent no-PR runs.** 2 of 10 runs ended COMPLETED with no PR and no last message. No restart:
  both tickets are low value for the current focus.
- **Implement run overreached.** 1086 shipped an unasked behaviour change (freeing names of
  finished sessions) and skipped the ticket's own check whether resume-by-name must keep working
  (`internal/subagent/session.go:159` returns the first name match). Caught in host review; PR
  left open.
- **Auto mode vs. the gear shim.** 9 of 10 first start calls were refused because the hook rewrote
  them after review (open 1112); resending the rewritten form worked.
- **Duplicate gate on wrappers.** `harnez issues new` blocked the batch wrappers as duplicates of
  the closed 1123-1135 wrappers (similar titles); `--force` was needed.
- **Draft counts as active.** 31 Draft wrappers raised the cap warning from 31 to 62 (filed 1172).

## Learnings

- Count tickets by every status before sizing a cleanup.
- An audit only pays when the code changed since the last one; check for any earlier Jules text,
  not one heading.
- Jules references are reliable (97%); its proposed next steps are not (1107 and 959 again
  proposed fixes the ticket had already rejected).
- Start the first run alone when GitHub `main` lags: it syncs, and parallel starts do not race.
- Batching by title is the only way 260 tickets fit in the days left before 2026-11-11.

## Follow-Up Tickets

- 1138: forge 404 hint should name `harnez forge token import`; stored token causes 403 on a
  public repo.
- 1139: `status:` filters for forge tickets.
- 1171: jules skill: skip recently audited tickets; add a batch kind and prompt.
- 1172: Draft tickets count toward the active-ticket cap.
- 1173: Jules config loader says "not valid JSON" for a structure mismatch
  (`internal/subagent/jules.go:275`).
- Open decision: 1086 fallback-only fix vs. PR #98.
