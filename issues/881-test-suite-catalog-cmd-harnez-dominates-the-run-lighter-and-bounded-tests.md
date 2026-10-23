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

## Baseline (host, 2026-10-23, `gotester report ./...`)

Step 1 needs no new target: `gotester report` runs each package alone and writes `testreport/report.md`
(git-ignored).

- Clean run (load low): 59 packages, 151.8s wall; `cmd/harnez` 88.8s wall, 183s CPU, 736 MiB; next
  `internal/usage` 12.1s, `internal/subagent` 10.2s, `internal/claude` 7.1s, all others under 3.5s.
- Process tests dominate `cmd/harnez` and scale with load (clean run → load 13 on 12 cores):
  `TestAgentWaitFailedWorkerIsNotOrphaned` 19s → 60s (fails under load),
  `TestForegroundStartAndResumeDetachIntoWaitableWorkers` 2.6s → 51s,
  `TestStopProviderTreeAndWrapper` 20s both (fails, 883). Under load `cmd/harnez` took 200s.
- Tests leak temp dirs: 153 stale `/tmp/harnez-test-*`/`go-build*` dirs filled the user's 9.5 GB `/tmp`
  quota and made the build fail ("disk quota exceeded"). Tests must use `t.TempDir()` or clean up.

The three process tests and their temp cleanup are fixed below; the remaining roadmap stays open.

## Process tests fixed

Measured on 2026-10-23, before/after in the same checkout session (seconds, Go test's per-test wall time):

- `TestStopProviderTreeAndWrapper`: before **20.08, failed all four variants**; after
  **3.06–3.20** across three repetitions; **5.73** with 12 CPU stress workers.
- `TestAgentWaitFailedWorkerIsNotOrphaned`: before **60.29, failed**; after **0.04–0.05**;
  **0.14** with stress.
- `TestForegroundStartAndResumeDetachIntoWaitableWorkers`: before **50.29**; after
  **0.29–0.33**; **0.47** with stress.

Cause: each re-executed test helper entered `TestMain`, created another isolated HOME/runtime,
and built the shared CLI again. Changing HOME also selected a cold Go cache. Providers had not
started when readiness expired: the empty PID list was not a tree-lookup miss. Killing wrappers
left their nested builds alive; the crash helper's `os.Exit` skipped all temp cleanup. Production
stop logic passed its existing wrapper/descendant/new-session assertions unchanged.

Fix: known helpers reuse their parent's isolated environment without allocating directories or
building binaries. The parent builds once using the caller's Go cache, and defers directory cleanup
so setup errors also clean up. Provider processes send readiness PIDs over sockets; worker helpers
wait for the parent's release after PID registration and detach assertions, replacing polling and
the completion sleep. Readiness and worker completion retain five-second watchdogs (race-scaled).

Verification:

- `go test ./cmd/harnez -run '^(TestStopProviderTreeAndWrapper|TestAgentWaitFailedWorkerIsNotOrphaned|TestForegroundStartAndResumeDetachIntoWaitableWorkers)$' -count=3 -v` passed.
  An enclosing private TMPDIR had **zero entries** after the run; the crash test also asserts
  that its abrupt helper creates no temporary directories.
- Same selection with `-count=1 -cpu=1` passed while 12 bounded CPU stress processes ran;
  all stress workers were terminated and joined afterward.
- Withholding the worker release deliberately failed in **5.06s**, with no helper temp directories.
- `make lint` passed. Single `make test-q1` passed, `cmd/harnez` **60.263s**;
  `grep -- '--- FAIL' /tmp/harnez-883-test-q1.log` found no failures. No untouched test was loosened.
- Logs: `/tmp/harnez-883-before.log`, `/tmp/harnez-883-final-repeat.log`,
  `/tmp/harnez-883-loaded.log`, `/tmp/harnez-883-test-q1.log` (local, not tracked).

## Temp directory leak found during 883 runs

Five `/tmp/harnez-test-*` directories created at 20:01:40–20:02:00 each contained a Go build
cache under the isolated HOME. The four provider-wrapper subprocesses were killed during stop-test
cleanup; the fifth crash-worker subprocess called `os.Exit`. Each subprocess had entered package
`TestMain`, which built the CLI into its fresh HOME before either termination path could run cleanup.

Fix: 883 commit `0ce47afc` makes the known subprocess helpers bypass isolated `TestMain` setup, builds
the shared binary before changing HOME, and defers cleanup for the parent test process. Its crash test
sets a private TMPDIR and asserts that the helper leaves it empty. Verified with
`go test ./cmd/harnez -run '^TestAgentWaitFailedWorkerIsNotOrphaned$' -count=1`: pass in 1.086s;
the enclosing TMPDIR was empty afterward.
