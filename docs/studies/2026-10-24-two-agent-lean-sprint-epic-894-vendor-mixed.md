# Two-agent lean sprint: epic 894, vendor-mixed workers (2026-10-24)

Four lean sprints over P1/P2 and value-add tickets (epic 894), with searcher, architect and developer from
different vendors where possible. The owner delegated all design decisions mid-run.

| Sprint | Tickets | Searcher → architect → developer | Commits | Result |
|---|---|---|---|---|
| 1 | 893 Haiku roles, 889 canary fixtures, 871 disk rule | luna:low → – → haiku:high | 7e40d615, 47b8a89d, f900c271 | closed all three; filed 895 |
| 2 | 866 release Unreleased rows, 887 kill diagnostics | haiku:low → – → sol:med | d5e3e492, 0ebe1cd8 | 866 closed; 887 open for M2 |
| 3 | 879 M2 collector polls remote load | haiku:low → sol:med → haiku:high | d1044f4e, c1973990 | closed |
| 4 | 851 bug-hunt, 852 tui-check, 880 jam skills | luna:low → opus:med → sol:med | be35d155, 1167ec07, 5b32605d | 880 closed; 851/852 wait for a first real run |

878 (retro skill) was skipped: it needs a new `harnez session extract` command.

## Observations

- Searchers resized the plan: 866 and 871 were not "easy" and each hid an owner decision. Splitting them out
  instead of forcing them into Sprint 1 kept Sprint 1 to two quick doc tickets.
- haiku:high handled 879 M2 (goroutine lifecycle, cache races, end-to-end fake-ssh test) well once a sol:med
  architect had fixed the boundaries. Its plan corrected three stale ticket claims on live code.
- haiku:high stopped correctly when a behaviour change broke `TestOneShotUsageDoesNotInvokeSSH` (the
  auto-started collector now polls over ssh) instead of loosening it. The fix scoped the assertion to the
  one-shot process; see ReviewLearnings "Daemon behaviour changes".
- Developers cannot run `harnez agent start` (leaf guard), so the 893 live check was the host's job; ticket
  acceptance that needs a live agent run should name the host as the checker.
- `harnez agent delete` refuses an unrated session; chain `rate` before `delete` in the host's teardown
  command, or the next command in the chain (here a developer start) never runs.
- Sprint 2's developer used its one q1 run, fixed a stale assertion after it, and did not rerun; the host
  reran q1 green. The one-run budget needs a "rerun after a post-run fix" exception or a host rerun.
- 879's live recovery could not be shown: x600 was offline ("No route to host"). Recheck when it is up.

## Token and time

Searcher turns 24–30 s; architect turns 60–80 s; developer turns 1.5–7 min. No rework rounds beyond one
blocker question (879) and one decision round per plan.
