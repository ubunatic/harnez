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
