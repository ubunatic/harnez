# 881 — Test suite catalog: cmd/harnez dominates the run, lighter and bounded tests

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Testing
**Related**: loom 377 (same roadmap there), 877 (features in sub-packages), 875 (resource warnings),
docs/TestTiers.md

---

/goal The harnez test suite tests the key behaviour without loading the machine or slowing developer turns:
a per-package report (wall time, CPU, peak RSS, slowest tests) exists, `cmd/harnez` no longer dominates the
run, and full runs are bounded; work goes one roadmap step at a time with before/after numbers; or stop and
report when blocked on a user decision.

## Finding (host, 2026-10-23, from existing logs, no test run)

- `make test-q1` of 873 M2 (`/tmp/harnez-873-m2-test-q1.log`): 51 packages ok; `cmd/harnez` 65.8s, next
  `internal/usage` 7.7s, `internal/subagent` 7.0s, `internal/claude` 4.0s, the rest under 2s each.
- `cmd/harnez`: 182 Go files, 100 of them tests, ~46k lines; almost all command logic and its tests live in
  the main package, so any change there reruns the 66s package (Go caches tests per package).
- 358 test files; `time.Sleep` in tests: heartbeat_test 7, watch_test 6, router_test 4, jobs_test 4,
  exec_test 4.
- Developer turns spend most time in test reruns: 873 M1 took 22 minutes with six `make test-q1` runs; 874's
  new watch PTY test hung about 4 minutes on fake-ssh teardown. 12 cores, `go test` without `-p` limit.

## Roadmap (one milestone per step, measure before/after)

1. **Report:** a make target that runs one package at a time, records wall, CPU and peak RSS
   (`/usr/bin/time -v`) and the 20 slowest tests (`go test -json`), into a short table. No full parallel run.
2. **Bound the load:** spec'd `-p` limit for `make test`/`test-q1`; per-test timeouts for binary/PTY tests so a
   hung child fails in seconds, not minutes.
3. **Split `cmd/harnez`:** an advisor proposes moving command logic and its tests into `internal/<feature>`
   packages (877), host + user review first; measure cache hits per typical change before and after.
4. **Sleeps:** replace with event waits or move to the full tier.
5. **Importance:** tag slow tests key / regression / redundant; merge or drop redundant ones after review.
