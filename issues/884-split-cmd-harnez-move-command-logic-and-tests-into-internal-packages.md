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
