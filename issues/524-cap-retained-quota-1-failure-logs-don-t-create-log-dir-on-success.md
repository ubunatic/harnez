# 524 — Cap retained quota-1 failure logs; don't create log dir on success

**Status**: Closed — 22595e3 + d40e522: logs capped, no dir on success; host suite passes
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Maintenance
**Related**: [[511-make-test-q1-keeps-the-full-test-log-and-prints-its-path-on-failure]], `cmd/harnez/exec.go`

## Problem

511 (400397c) writes one timestamped log per failed quota-1 run under
`.git/harnez/quota-1-logs/` and never prunes them. `quota1LogPath` also creates the
directory before the run, so successful runs create it too (flash37 review).

## /goal

Retained logs stay bounded (e.g. last 10), and a successful run touches no files.

## M1 delivered (22595e3), untested: host suite fails

`TestWriteQuota1FailureLogRetainsLatestTen` fails on HEAD. The developer used its
single test run before editing. harnez's 525 turn-end warning flagged both files.

## M2 Pre-Work / Required Refinements

- Fix the retention test or the pruning (whichever is wrong); don't weaken the assertion.
- Run the suite after the edit, not before.
