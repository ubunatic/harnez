# 524 — Cap retained quota-1 failure logs; don't create log dir on success

**Status**: Open
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
