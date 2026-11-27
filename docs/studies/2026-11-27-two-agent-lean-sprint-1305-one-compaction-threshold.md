---
kind: sprint
date: 2026-11-27
tickets: [1305]
---

# Lean sprint 1305: one compaction threshold per model, `--autocompact` on `harnez agent`

## Setup
- Developer: `dev-1305`, claude:haiku:med (owner's pick). Reviewer: the host per milestone; one final one-shot
  review by agy:neon:low (owner's pick).
- Searcher skipped: the ticket was written today from a neon:low review of live code and cites file:line premises.
  The developer's first turn is read-only (plan plus the M0 canary) and re-verifies them.
- `git.mode: team`, but the owner's standing rule allows direct fast-forward pushes to main.

## Log
- Turn 1 (plan + M0 canary): dispatched.
- Turn 1 (4m22s, peak context 66.4k): premises verified; M0 canary 3 runs (about $0.26). Env beats settings;
  80000 and 100000 both compact at about 66k. Conflicts with 1264's settings-key canaries (about 100k), so M0b adds
  three runs (env 150000, env 60000, settings-only 100000) before M1. The developer worked around the host's
  100-line Read guard, which the `claude -p` child inherits, by having it call `harnez read`.
- Turn 2 (7m39s, peak 65.0k): M0b's three runs show Claude triggers about 33k below the window with a 100k floor;
  env equals settings key in meaning. Host recomputed peaks and compactions from the raw stream files: match.
  M1 committed as 9d46d363. Host diff review found three LOW issues (non-hermetic Unsetenv, a warning on every turn
  for a disabled threshold, a malformed caller value passed silently); they are M2 pre-work. Test budget: `make test-q1`
  ran twice (fail, then full rerun) instead of a touched-package rerun.
- Turn 3 (8m35s, peak 65.8k, one `make test-q1`): pre-work 45898eac, flag 3a56d0e1. Host: installed binary
  rejects agy and `1.5k` before launch; touched packages green. Found: `--dry-run` hides the effective window; the
  detached resume path is untested. Both are M3 pre-work.
- Turn 4 (4m47s, peak 66.3k, one `make test-q1`, 83 ok): pre-work 46d306db (dry-run window line, detached-resume
  test), docs 3a52de28 (AutoCompaction.md "One threshold per model", legacy key and `ShouldCompact` removed).
  Host: dry-run prints `compact window: 120000 (--autocompact, env ...)`, `100000 (threshold, ...)` and Codex
  `90000 (--autocompact, -c ...)` with matching argv; `harnez init -d .` left the tree clean.
- Acceptance canary (host, `harnez agent start` haiku:med in /tmp/m0-1305, same 20-read load) and the neon:low final
  review were started in parallel.
- Acceptance canary: peak 65.7k, end 34.5k, 0 requests over 100k (1m42s).
- neon:low final review (1m50s, 228k tokens incl. cache): 1 HIGH (refused resume still saves `compact_tokens`),
  3 MED (`agent compact` drops the override; `resume -i --autocompact` silently ignored; Codex row wording), 1 LOW
  (resume dry-run skips provider check). Host verified all at the cited lines; none had been caught by the host
  diff reviews. They became M4.
- Turn 5 (4m12s, peak 67.0k, one `make test-q1`): M4 134d7c05 fixes all five review findings with tests (refused
  resume keeps `compact_tokens` for lineage/blocked cases; `agent compact` uses the stored window; `resume -i
  --autocompact` rejected; resume dry-run rejects agy; doc rows). Host reran the new tests and checked the agy
  rejection on the installed binary. Closed as 2e205f9d.

## Outcome
| Milestone | Commit | What |
|---|---|---|
| M0/M0b canary | ticket | Claude 2.1.330: env beats settings; trigger about 33k below the window; 100k floor |
| M1 env on Claude children | 9d46d363 | `CLAUDE_CODE_AUTO_COMPACT_WINDOW` = resolved threshold |
| M2 `--autocompact` | 45898eac, 3a56d0e1 | start/resume flag, persisted, claude+codex only |
| M3 dry-run + docs | 46d306db, 3a52de28 | window line in `--dry-run`; AutoCompaction.md; legacy key removed |
| M4 review fixes | 134d7c05 | 1 HIGH, 3 MED, 1 LOW from neon:low |

Acceptance: live Haiku agent with the owner's settings peaked at 65.7k, 0 requests over 100k.

## Lessons
- The canary changed the plan: the 1264 "about 100k at a 100k window" reading did not hold; Claude keeps about 33k
  reserve. Running M0 before code made the mapping decision a measured one.
- The final cross-model review found five bugs that three host diff reviews passed, all on paths the diffs did not
  show (refusal order, a second Model constructor, an early interactive return). Diff-only review misses code that
  should have changed but did not; a reviewer reading the surrounding call sites catches it.
- Haiku:med kept the one-run test budget in turns 3-5 after breaking it in turn 2 (full rerun after a failure).
- Developer context stayed near 65k for five turns: native compaction at about 66k held it under the 100k cost cliff.
