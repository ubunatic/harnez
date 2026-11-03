# Jules Through Harnez: Peer Feedback (2026-11-03)

Three Claude sessions on this machine used Google Jules through `harnez agent start --model jules` and
the `/jules` skill: voxi, lmcoder and loom. On 2026-11-03 each one answered the same five questions.
This study puts their answers next to the harnez fixes from that day (PRs #94-#100). The counts are
the sessions' own and were not re-checked here. Loom counted only its runs since its last context summary.

## Runs

| Kind | voxi | lmcoder | loom | Used |
| --- | --- | --- | --- | --- |
| implement | 1 | 5 finished, 2 running | 3 finished, 1 running | 7 of 9 finished |
| audit | 4 | 9 | 4 | 10 of 17 |
| ideas | 1 | 2 | 2 | 5 of 5 |
| experiment | 0 | 1 | 0 | 1 partly |

"Used" means the session adopted the PR or kept the report after checking its evidence. All 4 adopted
lmcoder implements needed one fix in review. voxi's implement needed none.

## What Works

- **Citations hold up.** Checked `path:line` references held 16 of 17 times (voxi), more than 90% of
  the time (lmcoder), and 8/8, 4/4, 4/5 and 5/5 in loom's kept reports. The verdict label is weaker:
  lmcoder rejected 3 audit verdicts, and Jules said "partly done" for tickets where nothing was done.
- **Ideas runs give the most per run.** Every ideas run was used. They found a data race and a health
  check that can never fail (lmcoder), and led to 4 quick fixes and a ticket (loom). Narrow targets
  worked best.
- **Audits work as a stale-ticket sweep.** Audits closed tickets (lmcoder's 057 and 060) and gave real
  starting points for others.
- **Implement runs are good drafts** for small, well-specified tickets that come with a test.
- **The GitHub sync on start** (issue 1074) meant no session pushed GitHub by hand for Jules.
- **The split between `resume wait` and `status --id`** (loom) made adopting and reading runs simple.

## What Costs Time

| Problem | Reported by | State |
| --- | --- | --- |
| Waits run out (about 25 min) and exit 0 while Jules is still running, so each run needs 1-2 resumes | all three | 1111 open, needs an owner decision |
| Jules asks back (7 of 14 lmcoder runs) | lmcoder, loom, voxi | Auto-reply merged (1113). First live case (lmcoder run 130): it was sent 24 s after the question, but Jules did not react within 25 min; only a reply that answered the actual question moved the run on. The auto-reply showed only on stderr, so its result now also says so on stdout |
| The ⚙ hook rewrite makes auto mode refuse the call and its batch | all three | 1112: timeout part fixed in PR #100, refusal open |
| The 1m exec timeout killed a start | lmcoder | Fixed in PR #100 |
| A failed start kept its name; names are global across repos | voxi, loom | 1108 frees failed names; global names stay, and the error now says so |
| FAILED showed no reason | voxi | Fixed in PR #95 |
| `git pull`/`git fetch` replaced FETCH_HEAD before the Jules PR was read, giving an empty report | voxi, loom | Open, see wishes |
| Weak Jules work behind a `done` verdict: tests always passing, failing or using made-up data; a golden rewrite; voxi's implement 220 said `done` but left its main item unchanged and compared error strings | lmcoder, loom, voxi | Skill now asks for a failing run first and for real formats |
| About 6 manual steps per finished run | lmcoder | Open, see wishes |
| harnez's running count left out a run Jules still showed IN_PROGRESS | lmcoder | Not checked yet |
| A GitHub collaborator invite stayed pending until accepted through the gh API | voxi | Setup step, not harnez |
| Codeberg AGit pushes failed for 30-60 min ("fail to run proc-receive hook") | loom | Forge outage; a retry loop risks duplicate PRs |

## Wishes

Different sessions asked for the same few things:

1. **Collect a finished run in one command** (lmcoder `jules collect`, voxi `jules fetch --apply`,
   loom `jules fetch-report`): find the PR, check its scope, write `reports/jules/<n>.md` from the PR
   head without FETCH_HEAD, add the ticket's Jules-runs row, then close the PR and delete its branch.
   This removes the FETCH_HEAD problem and most of the manual steps.
2. **Wait until the run ends** (loom `--until done`; all three resume repeatedly). This depends on the
   1111 exit-code decision.
3. **Spaced batch starts** (voxi `--after 2m`; lmcoder batch start with one notice per run). This also
   avoids `sleep` wrappers, which the ⚙ hook changes.
4. **`harnez pr wait` for several PRs** (loom).

## `harnez pr wait`

Only loom uses it, as the background gate before fast-forwarding main and syncing GitHub. voxi and
lmcoder push main directly, and Jules PRs are fetched and closed, never merged. So `pr wait` fits
repos with PRs, not the Jules collect step.

## Takeaways

- Treat Jules's evidence as reliable and its verdicts and tests as drafts. The skill's evidence check
  is the right gate; keep it.
- Waiting and collecting cost the most, not the Jules work itself. Wishes 1 and 2 are the
  highest-value harnez changes; wish 2 needs the 1111 decision first.
- Prefer ideas runs on narrow areas, and implement runs only for small tickets with a test.
