# 884 — Split cmd/harnez: move command logic and tests into internal packages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactor
**Related**: 881 (test suite catalog, step 3), 877 (practice: features in sub-packages), 883, loom 378 (same move in loom)

---

/goal `cmd/harnez` keeps only flags, help, wiring and exit codes; command logic and its tests live in
`internal/<feature>` packages, so a typical change reruns a small package instead of the 66s `cmd/harnez`
tests, and tests call Go functions instead of the built binary wherever the binary isn't the thing under
test; measured before/after; or stop and report when blocked on a user decision.

## Findings (host, 2026-10-23)

- `cmd/harnez`: ~22k lines of non-test code, ~46k lines with its 100 test files; its tests take 65.8s of a ~4 min
  `make test-q1`. Go caches test results per package, so any change in the package reruns all of them.
- Largest files: exec.go 1405, issues.go 1326, agent_run.go 1191, hook.go 1174, stats_agents.go 759,
  agent.go 728, issues_rebase.go 716.
- 13 test files build or start real binaries (`exec.Command`). In loom the suite also got slow when tests
  ran real CLI binaries from the main package.

## Plan (one move per milestone, pure moves, measure each)

1. An advisor proposes the package boundaries and order (biggest test-time win first), and which binary
   tests become in-process tests; host and user review it.
2. Move one feature per milestone (for example issues, exec, hook, agent) with its tests; behaviour
   unchanged; record `cmd/harnez` test time and the tested packages for a typical change.
3. Keep a few end-to-end binary tests in the larger, occasional tier (881).

## M1 delivered: mechanical moves (98d1f6d4, 8a431215, 9e567c93)

find → `internal/find`, part of issues → `internal/issues/maintenance`, part of usage → `internal/usage`.
`cmd/harnez` 182 → 179 files, ~45.9k → 43.7k lines. The rest mixes Cobra setup with logic and shares
main-package helpers, so it cannot move without boundary changes. `TestAgentWaitFailedWorkerIsNotOrphaned`
failed once in a loaded 206s run and passes alone before and after the moves (timing, not the move).

## Boundaries (advisor adv-884, 2026-10-23)

Order keeps `go build ./...` green after each step: **exec → stats → hook → issues core → usage → agent**.
Every area keeps its Cobra command, flags, argument parsing and exit handling in `cmd/harnez`; logic takes
context, writers and dependencies as parameters, no `*cobra.Command` in internal packages.

| Area → package | Moves | Est. logic lines touched | Risk |
|---|---|---|---|
| exec → `internal/exec` | options, wrapper, timeout/sandbox/quota, telemetry, buffers, hook rewriting (`exec.go`) | 250–450 | medium-high |
| stats → `internal/stats` | options, queries, calculations, renderers (`stats*.go`, `statsmergelegacy.go`) | 120–250 | medium |
| hook → `internal/hook` (+ `internal/sound`) | payload handling, read discipline, pre-edit/pre-exec, skills (`hook.go`, `hookskill.go`) | 200–350 | medium |
| issues → existing `internal/issues` | status, reserve, duplicate, reference, ticket-file logic; index/snapshot ops injected | 300–600 | medium-high |
| usage → `internal/usage` + new `internal/usagecli` (adapters) | `usage_client.go`, `usage_timeline.go`, non-Cobra parts of `usage_cmd.go`, export/archive | 350–700 | high |
| agent → new `internal/agent` on `internal/subagent` | run/resume, lifecycle, stream, quota, send, self-test (`agent_run/async/interactive/send/stop/stream/quota/selftest.go`) | 900–1,500 | high |

Shared helpers: `exec.go:1372 debugLog` (also used by index.go) → small `internal/logging` or injected
logger; `exec.go:59 defaultExecTimeout` (used by stats_agents.go) → exported from `internal/exec`;
`index.go:85 dirExists` → `internal/fsutil` or a callback; `agent.go:507 newTimeline`,
`agent_run.go:123 agentCommandContext/withAgentTimeout` → pass context and writers.

Cycles: `internal/issues` must not import `internal/index`, `internal/find` or `internal/issues/maintenance`
(they import it); `internal/usage` must not import `internal/usagecli`; nothing imports `cmd/harnez`.

Binary tests stay in `cmd/harnez` or go to the occasional tier: `exec_test.go:969`,
`issues_index_init_merge_e2e_test.go:16`, `usage_binary_test.go`.

## M2 delivered: exec move

Code commit: `2df4d74d` (`refactor(exec): move wrapper logic into internal package (issue 884)`).
The execution wrapper, timeout/sandbox/quota behavior, telemetry, buffers, hook rewriting, stopped
process-group helpers, and their unit tests now live in `internal/exec`. Cobra command construction,
flags, and exit handling stay in `cmd/harnez`; the three command/binary tests remain there. The
package boundary has no Cobra import. Shared `debugLog` now delegates to `internal/logging`, and
`DefaultExecTimeout` is exported for the upcoming stats move.

Measured with `go test -count=1` on 2026-10-23:

- Before: `cmd/harnez` **56.485s**.
- After: `cmd/harnez` **64.063s**; `internal/exec` **5.006s**. The cmd package was under host load,
  so this run does not show an immediate wall-time reduction; ordinary changes in moved logic can
  now test only `internal/exec`.
- `HTO=0 make test-q1` passed once; `grep -- '--- FAIL' /tmp/harnez-884-test-q1.log` found no failures.
- `make lint` passed. No boundary violations found. `stats_agents.go` can consume the exported
  timeout in the stats milestone; no `*cobra.Command` entered the internal package.

The preceding issue 881 temp-leak documentation commit is `b4980a1b`.

## M3 delivered: reuse built binaries and Go caches

`cmd/harnez` test baseline on 2026-10-23 was **48.758s**; after M3 it was **25.906s**,
a **22.852s** reduction. Commit: `7c6a5e0a`.

`TestGearMulticallExecution` and issue-merge E2E now share one lazily built test binary, preserving
their real process, argv[0], exit-code, and Git-merge assertions. TestMain resolves and preserves
absolute `GOCACHE`/`GOMODCACHE` paths before isolating HOME, and short-tier runs no longer build a
binary eagerly. `make lint` passed; only the repository's existing warning summary remains.

## M4 delivered: usage test tiers and in-process status matrix

The post-M3 `cmd/harnez` baseline was **25.066s**; after M4 it was **21.349s**, a **3.717s**
reduction. The six usage-server status cases now use injected probe decisions and assert status
writer routing in-process. The FIFO overlay build was replaced with a bounded store-read test
using an injected reader and controllable context cancellation. `TestUsageBinaryReturnsWithinDeadline`
retains one real-binary hung-server watchdog; the SSH safety and PTY lifecycle subprocess tests also
remain. All usage binary tests are full-tier, and TestMain builds lazily, so short runs do not build
the CLI.

Verification on 2026-10-23: `go test -short ./cmd/harnez -count=1` passed in **10.347s**; `make lint`
passed; the single `make test-q1` passed (`cmd/harnez` **21.224s**) and `grep -- '--- FAIL'`
found no failures. No `/tmp/harnez-test-*` directories remained after the run.

## Plan v2 (advisor adv-884b)

### Review and measured baseline (2026-10-23)

- Brief review of `2df4d74d`: no material CLI behaviour regression found. Timeout, exit,
  sandbox, rewrite and logging logic are preserved. Agent detection and telemetry sending
  are now explicit dependencies; `cmd/harnez/exec.go:execDependencies` supplies both on
  wrapper/record paths. Direct internal callers must supply them: an empty AgentID stays
  empty, and nil SendTelemetryRecord with no DBPath returns the unavailable error.
  Detection now happens before execution rather than during telemetry recording.
- No Cobra import/type or reverse cmd import in `internal/exec` or `internal/logging`;
  `go list -deps ./cmd/harnez` succeeds (no reachable import cycle). Graph generation
  `2026-10-23T18:19:24Z`, coverage checked for both packages and cmd; source diff checked too.
  The compatibility forwarding API is large: trim unused exports/forwarders during later
  moves, and have shared logging callers import `internal/logging` directly.
- Single requested run: `HTO=0 go test ./cmd/harnez -count=1 -json`, **PASS, 49.131s**;
  raw JSON `/tmp/harnez-adv-884b-tests.json` (local, untracked). 579 passing top-level tests
  total **48.04s** (rounded JSON elapsed values); roughly **1.09s** remains for setup/cleanup.
  Package start to first test was **0.89s**, including TestMain's shared build. This is
  one host/cache observation, not a controlled comparison with 56.485s or 64.063s.
- Rank top-level tests only: parent elapsed includes subtests; never add parent and child
  timings. Categories below distinguish processes, real waits, and in-process logic;
  the latter includes temp-file/DB work, not strictly pure computation.

### Twenty slowest tests (seconds)

| # | Test | Seconds | Group |
|---|---|---:|---|
| 1 | TestGearMulticallExecution | 24.68 | binary/build/process |
| 2 | TestUsageBinaryStoreReadDeadlinePrintsError | 4.28 | binary/overlay build + deadline |
| 3 | TestStopProviderTreeAndWrapper | 2.98 | real process trees/termination grace |
| 4 | TestUsageBinaryReturnsWithinDeadline | 2.73 | binary/socket deadlines |
| 5 | TestInitInstallsIssueMergeDriverEndToEnd | 1.75 | binary/build/Git |
| 6 | TestUsageWatchShowsHungServerStatusPTY | 1.24 | binary/PTY/socket deadline |
| 7 | TestAgentSelftestFullSequenceSuccess | 1.00 | in-process, real 1s timer + polling sleeps |
| 8 | TestBenchRunCardModeForcesImageRead | 0.29 | in-process fake driver/files |
| 9 | TestForegroundStartAndResumeDetachIntoWaitableWorkers | 0.28 | real helper processes |
| 10 | TestBenchRunModelReadMatrixAndLiteDefault | 0.28 | in-process fake driver/files |
| 11 | TestIssuesCloseAlreadyClosed | 0.25 | in-process command/Git fixture |
| 12 | TestAgentResumeWithoutNameResolution | 0.21 | in-process fake driver/store |
| 13 | TestAgentSelftestVerifyMissingHTOFails | 0.20 | in-process timer/polling sleeps |
| 14 | TestIssuesCloseBatchPreservesClosure | 0.19 | in-process command/Git fixture |
| 15 | TestLeafRolesCannotStartOrManageAgents | 0.17 | in-process fake driver/store |
| 16 | TestDocsCardsCmd_BuildAndCheck | 0.17 | in-process rendering/files |
| 17 | TestDuplicateNumbersRejectIndexAndStatus | 0.14 | in-process command/Git fixture |
| 18 | TestIssuesRemoteAllocation | 0.14 | in-process command/fake gh process |
| 19 | TestSilentAgentGetsWarning | 0.13 | in-process fake driver with sleep |
| 20 | TestOrchestratorMayStartHelpersButNotOrchestrators | 0.12 | in-process fake driver/store |

Top six account for **37.66s (77% of package time)**. The worst remaining build is
`exec_test.go:49`: TestMain already built once with the caller's cache, but this test
builds again after HOME isolation, selecting a fresh Go cache. The issue merge E2E
and usage overlay build also invoke Go after HOME isolation. Sleeps are secondary.

### Ordered implementation steps and expected gains

Estimates below require before/after confirmation; do not sum overlapping gains. Moving
tests between packages improves cache scope and cmd time, not total work in an uncached
full run. `docs/TestTiers.md` defines short versus full; “occasional” means full-tier
developer runs, with full and race coverage retained in CI, not silently dropped tests.

1. **Fix build ownership before another feature move.** Reuse TestMain's shared binary
   for multicall (symlink it) and issue-merge E2E; preserve the caller's resolved absolute
   GOCACHE/GOMODCACHE for the overlay build before changing HOME. Keep application HOME
   isolated. Build lazily only when a binary test asks, with a bounded context and cleanup.
   Expected full cmd saving **22–27s**, principally the measured 24.68s cold multicall build;
   ordinary `-run` and short runs also avoid the current ~0.9s setup. Do not merely move
   the cold build to another package. Retain real argv[0], exit-code and Git-hook assertions.
2. **Make tiers effective and convert usage matrices.** Multicall, merge-driver E2E,
   stop-tree, foreground-worker, PTY and real usage deadline tests belong to full; several
   already call skipShort. Add the missing short guard to `TestOneShotUsageDoesNotInvokeSSH`
   and ensure TestMain never eagerly builds for short runs. Keep a few actual subprocess
   tests for exit, FIFO/process deadlines, PTY lifecycle and merge installation. Move the
   six-case usage status matrix to in-process functions with injected probe/store/clock;
   replace the overlay-build test's broad logic coverage with an injected store deadline,
   retaining one full binary watchdog regression. Expected **~3–5s additional full-run
   saving** from the matrix after step 1; short tier avoids the remaining **~7–10s** of
   process/PTY/Git work (overlaps the matrix saving). Full binary watchdogs remain bounded.
3. **Usage logic first, then issues core, for test-time/cache payoff.** Current test-file
   totals by prefix: usage **8.41s**, issues **3.93s** (includes 1.75s E2E). Extract usage
   read/probe/render/export orchestration to existing `internal/usage` or a Cobra-free
   `internal/usagecli`; pass ctx, writers, store/probe interfaces. Move issues ticket/status/
   reserve/reference logic to existing `internal/issues`, injecting index/snapshot work to
   avoid the index/find/maintenance → issues cycle. Keep CLI wiring/E2E in cmd. After the
   preceding optimizations, expect **~1–3s usage and ~2s issues** of cmd runtime relocation,
   **~2.5k current non-test lines** removed from cmd context together; no equivalent total
   full-suite saving from relocation alone. Feature edits then rerun only their package.
4. **Replace real time, then extract agent in bounded slices.** `agent_stream_test.go:58`
   uses 60ms per fake turn for heartbeat ordering; use a controllable ticker and emission
   barrier. `agent_protocol_test.go:33` uses delayed scripted replies; trigger the warning
   via fake clock before releasing the reply. Replace selftest's 1s/200ms timers and
   20ms/10ms polling (`agent_selftest_test.go:133,222`) with injected timer/start/completion
   events. Replace worker registration polling (`agent_wait_test.go:474`) with a parent
   release acknowledgment. Expected **~1.5–2s real-time saving**, reduced timing flakiness.
   Extract lifecycle/stream/selftest into `internal/agent` over `internal/subagent`, keeping
   command construction outside and eliminating package-global test hooks. Current agent
   tests total **8.41s**, including the 2.98s stop-tree and 0.28s foreground fixtures that
   stay full-tier; after timers/tiers, expect **~3–4s** fast tests relocated and **~5k lines**
   less cmd context. This is the largest context win but highest-risk move, not the first
   quick latency fix; preserve process identity, descendant shutdown and join assertions.
5. **Stats, then hook: context work, low latency priority.** Stats tests total **0.25s**,
   hook tests **0.06s**; moves cannot plausibly remove seconds of full-suite execution.
   Extract ~1.8k/~1.4k current source lines respectively with ctx/options/writers and no
   Cobra types. Stats consumes the exec timeout export; sound moves separately if needed.
   Replace hook sound's 20ms log polling and detached shell `sleep 10` fixtures with a
   readiness channel/socket, release barrier and joined cleanup. Expected saving **<0.1s**
   in reported test time, plus removal of lingering ten-second children; keep actual sound
   spawn coverage full-tier. These moves primarily shrink context and package cache scope.
6. **Verify each milestone once, record comparable profiles.** Capture `-json` top-level
   totals, short/full timing, build count and cache paths, and affected-package cache hits
   under the same load; widen using TestTiers before commit. Expected outcome after build/
   timer/matrix fixes: cmd full **roughly 18–25s before further relocations**, short lower;
   this is a target, not a measured result. Pure moves may reduce cmd further while leaving
   uncached full-suite work unchanged. No new reporting target is needed (881 has gotester).

## M5 delivered: deterministic readiness and tiered timer tests

The detached worker helper now waits on the existing parent-release socket event after the
parent confirms the worker PID is registered; it no longer polls session state every 10ms.
Self-test start/completion coordination uses test-controlled channels instead of polling and
sleeping for the configured demonstration duration. Client heartbeat expiry tests use an
injected clock. The jobs UI-loop helper wakes on posted callbacks rather than polling; tests
whose assertions require real process trees, elapsed time, or rate-limit timers are explicitly
full-tier. The slow telemetry test releases and joins its deliberately blocked writer instead
of leaving a 10-second sleeper behind.

Measured `go test ./cmd/harnez -count=1`: **41.384s before**, **44.326s after**. Both runs
failed in the untouched `TestAgentQuotaNoticeBeforeStartAndResumeJSON/start`, which did not
observe its quota warning before the provider turn. Touched-package verification passed:
`internal/usage` **14.063s**, `internal/tui/router` **0.122s**, `internal/tui/jobs` **4.103s
(-count=3)**, and `internal/exec` **8.357s**. The post-M5 targeted cmd tests passed in **0.231s**
(-count=3). `make lint` passed. Remaining real-time process and heartbeat/watchdog assertions
stay in full tier by design.
