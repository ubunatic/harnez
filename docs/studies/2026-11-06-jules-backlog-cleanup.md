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
| 1 | 1140-1149 | done 01:22: 10/10 delivered a PR, 3 needed one resumed wait (>25m) |
| 2 | 1150-1159 | done 02:15 on `2cf3be4d`: 8 delivered, 6 needed a resumed wait, 1152 and 1157 stuck (>50m, dropped) |
| 3 | 1160-1169 | pending |
| 4 | 1170 | pending |

Wave 1 result (87 tickets):

| Batch | Refs held | Proposed closes | Outcome |
| --- | --- | --- | --- |
| 1140 epics | 6/6 | 0 | 8 keep |
| 1141 Jules | 6/6 | 3 | all rejected |
| 1142 research 1 | 10/10 | 5 | 302 closed; 4 rejected |
| 1143 research 2 | 2/3 | 1 (842 already closed) | discarded |
| 1144 release | 2/3 | 2 | discarded; both rejected |
| 1145 git/forge 1 | 5/5 | 4 | all rejected; 1040 noted partly done |
| 1146 git/forge 2 | 3/3 | 0 | 7 keep |
| 1147 issues 1 | 2/5 | 2 | discarded; 615 verified done, close blocked (1174) |
| 1148 issues 2 | 6/6 | 3 (843 already closed) | 702 closed |
| 1149 issues 3 | 2/4 | 1 | discarded; rejected |

21 proposed closes: 2 closed (302, 702), 1 verified but blocked (615), 2 already closed, 16
rejected. 4 of 10 reports discarded (<80% refs). Rejections share one cause: Jules matched the
title against existing code and skipped the ticket's done-when (1015 path vs. number rewrite,
246 /commit without the ownership rule, 1040 without completion, 516 still "unmeasured").

Deinvest leads (Jules priority `drop`, for the owner): 230 (Android release), research 518, 619,
763, 914, 960, 1007, 1094, and 350.

Prompt change for waves 2-4: a preamble names the role (strict backlog auditor), the approach
(done-when first: list the `/goal`, `Want` and `Done when` items, cite each) and the task, and a
`close: done` must quote each required item with the code that does it.

Wave 2 result (stricter prompt):

| Batch | Refs held | Proposed closes | Outcome |
| --- | --- | --- | --- |
| 1150 usage 1 | 2/6 | 1 | discarded; 160 rejected (questions answered with function names) |
| 1151 usage 2 | 4/5 | 2 | both rejected; 587 noted partly done |
| 1152 usage 3 | - | - | stuck, dropped |
| 1153 usage 4 | 3/4 | 1 | discarded; 927 unchecked (no gokay on host) |
| 1154 models 1 | unscored | 1 | 656 rejected (M2, M3 missing) |
| 1155 models 2 | 6/6 | 1 | 755 verified |
| 1156 skills 1 | invented quote | 2 | discarded; 233 closed on host check, 286 rejected |
| 1157 skills 2 | - | - | stuck, dropped |
| 1158 skills 3 | 6/8 | 2 | discarded; 617, 736 verified |
| 1159 skills 4 | 5/8 | 1 | discarded; 926 noted partly done |

11 proposed closes, 4 held (233, 617, 736, 755): 36% against 10% in wave 1. Cost: 6 of 10 runs
ran past 25 minutes (wave 1: 3), and 1156 invented a quote (`docs/lang/Go.md:27` "Use
golang.org/x/term"; Go.md has no `x/term`). Kept the prompt for waves 3-4: every close is checked
by the host anyway, so the higher hit rate matters more than the extra noise.

Close gate: 615, 617, 736 and 755 were done in commits that do not name them. 1174 added
`issues close --implemented-by <sha>` (`74e2ca78`, test `cmd/harnez/issues_implemented_by_test.go`);
all four closed with it.

Closed so far: 302, 702, 233, 615, 617, 736, 755 (plus 1174).

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
- **Status filter matched prose.** My backlog filter matched `Backlog` anywhere in the Status line,
  so 842, 843 and 1083 (Closed, with "Backlog" in their outcome text) entered batches. Harmless;
  match a leading `Backlog` instead.
- **Batch close claims.** 16 of 21 proposed closes in wave 1 were wrong (see table). Caught by
  checking each against the ticket's goal; prompt tightened for waves 2-4.
- **Close gate vs. old work.** 615 is done in `ca8bada1`, which does not name it; `issues close`
  refuses without `--no-code`, which would be false (filed and shipped as 1174).
- **Invented quote.** 1156 quoted a line that does not exist anywhere in the cited file. The
  per-reference check caught it; one invented quote discards the whole report.
- **Reference format drift.** Reports wrote references as `path:line`, `path:file:line` or bare
  `file:line` without a quote; the host's checker script had to accept the variants, and unquoted
  references stay unscored.

## Learnings

- Count tickets by every status before sizing a cleanup.
- An audit only pays when the code changed since the last one; check for any earlier Jules text,
  not one heading.
- Jules references are reliable (97%); its proposed next steps are not (1107 and 959 again
  proposed fixes the ticket had already rejected).
- Start the first run alone when GitHub `main` lags: it syncs, and parallel starts do not race.
- Batching by title is the only way 260 tickets fit in the days left before 2026-11-11.
- A batch verdict is a lead per ticket, not a decision: in wave 1, 2 of 21 proposed closes held.
  Check each close against the ticket's done-when, and run it live where cheap (702, 615).

## Follow-Up Tickets

- 1138: forge 404 hint should name `harnez forge token import`; stored token causes 403 on a
  public repo.
- 1139: `status:` filters for forge tickets.
- 1171: jules skill: skip recently audited tickets; add a batch kind and prompt.
- 1172: Draft tickets count toward the active-ticket cap.
- 1173: Jules config loader says "not valid JSON" for a structure mismatch
  (`internal/subagent/jules.go:275`).
- 1174 (shipped): `issues close --implemented-by <sha>` for work done under an unreferencing commit.
- Open decision: 1086 fallback-only fix vs. PR #98.
